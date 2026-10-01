package kbapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/rag/index"
	"github.com/linkerlin/agentscope.go/rag/kb"
)

// Handlers carries the narrow dependency surface of the KB HTTP API: only the
// KBService. Register mounts every route; the gateway root passes its mux and
// auth wrapper in, which keeps URL decisions in one place while the handler
// logic stays independently testable.
type Handlers struct {
	Svc *KBService
}

// NewHandlers builds the KB handlers for one attached service.
func NewHandlers(svc *KBService) *Handlers { return &Handlers{Svc: svc} }

// AuthWrapper wraps a handler with the caller's authentication middleware.
// Pass nil in Register to skip authentication (tests).
type AuthWrapper func(http.HandlerFunc) http.HandlerFunc

// Register mounts the knowledge-base CRUD + upload + search routes on mux.
func (h *Handlers) Register(mux *http.ServeMux, auth AuthWrapper) {
	wrap := auth
	if wrap == nil {
		wrap = func(h http.HandlerFunc) http.HandlerFunc { return h }
	}
	mux.HandleFunc("GET /api/v1/knowledge-bases", wrap(h.listKnowledgeBases))
	mux.HandleFunc("POST /api/v1/knowledge-bases", wrap(h.createKnowledgeBase))
	mux.HandleFunc("GET /api/v1/knowledge-bases/{id}", wrap(h.getKnowledgeBase))
	mux.HandleFunc("DELETE /api/v1/knowledge-bases/{id}", wrap(h.deleteKnowledgeBase))
	mux.HandleFunc("POST /api/v1/knowledge-bases/{id}/documents", wrap(h.uploadDocument))
	mux.HandleFunc("GET /api/v1/knowledge-bases/{id}/documents", wrap(h.listDocuments))
	mux.HandleFunc("DELETE /api/v1/knowledge-bases/{id}/documents/{doc_id}", wrap(h.deleteDocument))
	mux.HandleFunc("GET /api/v1/knowledge-bases/{id}/documents/{doc_id}/chunks", wrap(h.listDocChunks))
	mux.HandleFunc("GET /api/v1/knowledge-bases/{id}/documents/{doc_id}/raw", wrap(h.rawDocument))
	mux.HandleFunc("POST /api/v1/knowledge-bases/{id}/search", wrap(h.searchKnowledgeBase))
}

// --- request / response types ---

type createKBRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	EmbedderID  string         `json:"embedder_id"`
	Filter      map[string]any `json:"filter,omitempty"`
}

type searchKBRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}

type searchResultItem struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Score    float64        `json:"score"`
	Source   string         `json:"source"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// --- handlers ---

func (h *Handlers) listKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	specs := h.Svc.Manager.List(r.Context())
	// ponytail: per-KB ListDocuments aggregate is O(records); fine for the
	// in-memory store — move into the manager when remote backends arrive.
	type kbSummary struct {
		kb.Spec
		Documents int `json:"documents"`
		Chunks    int `json:"chunks"`
	}
	out := make([]kbSummary, 0, len(specs))
	for _, spec := range specs {
		sum := kbSummary{Spec: spec}
		if handle, err := h.Svc.Manager.Get(r.Context(), spec.Name); err == nil {
			if docs, err := handle.ListDocuments(r.Context()); err == nil {
				sum.Documents = len(docs)
				for _, d := range docs {
					sum.Chunks += d.Chunks
				}
			}
		}
		out = append(out, sum)
	}
	writeJSON(w, http.StatusOK, map[string]any{"knowledge_bases": out})
}

func (h *Handlers) createKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	var req createKBRequest
	if err := decodeJSONLimit(w, r, &req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	spec := kb.Spec{
		Name:        req.Name,
		Description: req.Description,
		EmbedderID:  req.EmbedderID,
		Filter:      req.Filter,
	}
	if err := h.Svc.Manager.Create(r.Context(), spec); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (h *Handlers) getKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	handle, err := h.Svc.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	docs, _ := handle.ListDocuments(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        handle.Name,
		"description": handle.Description,
		"documents":   docs,
	})
}

func (h *Handlers) deleteKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	if err := h.Svc.Manager.Delete(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) uploadDocument(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	handle, err := h.Svc.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	docID, mediaType, source, data, err := readUpload(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	blobKey := fmt.Sprintf("%s/%s", name, docID)
	uri, err := h.Svc.Blob.Put(ctx, blobKey, strings.NewReader(string(data)))
	if err != nil {
		http.Error(w, fmt.Sprintf("blob put failed: %v", err), http.StatusInternalServerError)
		return
	}

	task := index.Task{
		KBName:    name,
		DocID:     docID,
		BlobURI:   uri,
		MediaType: mediaType,
		Source:    source,
	}
	var status index.Status
	h.Svc.Worker.OnStatus = func(s2 index.Status) { status = s2 }
	if err := h.Svc.Worker.Process(ctx, task); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"doc_id": docID, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"doc_id":   docID,
		"source":   source,
		"chunks":   status.Chunks,
		"kb_name":  name,
		"document": handle, //nolint — keep handle in response for traceability
	})
}

func (h *Handlers) listDocuments(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	handle, err := h.Svc.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	docs, err := handle.ListDocuments(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

// listDocChunks returns every indexed chunk of one document (no vectors),
// ordered by chunk index.
func (h *Handlers) listDocChunks(w http.ResponseWriter, r *http.Request) {
	handle, err := h.Svc.Manager.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	recs, err := handle.ListChunks(r.Context(), r.PathValue("doc_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if recs == nil {
		recs = []kb.Record{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"chunks": recs})
}

// rawDocument streams the original uploaded bytes of a document from the blob
// store (addressed via the blob_uri tagged on its chunks).
func (h *Handlers) rawDocument(w http.ResponseWriter, r *http.Request) {
	handle, err := h.Svc.Manager.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	docID := r.PathValue("doc_id")
	recs, err := handle.ListChunks(r.Context(), docID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var blobURI string
	if len(recs) > 0 {
		blobURI, _ = recs[0].Metadata["blob_uri"].(string)
	}
	if blobURI == "" {
		http.Error(w, "no raw blob recorded for document "+docID, http.StatusNotFound)
		return
	}
	rc, err := h.Svc.Blob.Get(r.Context(), blobURI)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if name, _ := recs[0].Metadata["source"].(string); name != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", name))
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	_, _ = w.Write(data)
}

func (h *Handlers) deleteDocument(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	docID := r.PathValue("doc_id")
	handle, err := h.Svc.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := handle.DeleteDocument(r.Context(), docID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) searchKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	handle, err := h.Svc.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var req searchKBRequest
	if err := decodeJSONLimit(w, r, &req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	results, err := handle.Search(r.Context(), req.Query, req.TopK)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items := make([]searchResultItem, 0, len(results))
	for _, res := range results {
		src, _ := res.Metadata["source"].(string)
		items = append(items, searchResultItem{
			ID: res.ID, Text: res.Text, Score: res.Score, Source: src, Metadata: res.Metadata,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": items})
}

// --- helpers ---

// newDocID mints a unique document id (same shape as the gateway root's
// generateID, kept local so this package has no gateway dependency).
func newDocID() string {
	return fmt.Sprintf("doc-%d", time.Now().UnixNano())
}

// readUpload extracts document bytes from either a multipart/form-data upload
// or a JSON body. Returns (docID, mediaType, source, data, err).
// maxKBJSONBody caps the JSON upload fallback (18.11); the multipart path
// keeps its own explicit size checks.
const maxKBJSONBody = 1 << 20

// decodeJSONLimit decodes a JSON body under the package cap; crossing it
// stops reading at the boundary and returns *http.MaxBytesError.
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxKBJSONBody)
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func readUpload(w http.ResponseWriter, r *http.Request) (string, string, string, []byte, error) {
	docID := newDocID()
	if err := r.ParseMultipartForm(32 << 20); err == nil {
		// multipart: read the "file" field
		if f, hdr, err := r.FormFile("file"); err == nil {
			defer f.Close()
			data, err := io.ReadAll(f)
			if err != nil {
				return "", "", "", nil, err
			}
			source := hdr.Filename
			if source == "" {
				source = "upload"
			}
			mt := hdr.Header.Get("Content-Type")
			if mt == "" || mt == "application/octet-stream" {
				mt = mediaTypeFromExt(source)
			}
			if id := r.FormValue("doc_id"); id != "" {
				docID = id
			}
			return docID, mt, source, data, nil
		}
	}
	// JSON fallback
	var body struct {
		DocID     string `json:"doc_id"`
		Content   string `json:"content"`
		MediaType string `json:"media_type"`
		Source    string `json:"source"`
	}
	if err := decodeJSONLimit(w, r, &body); err != nil {
		return "", "", "", nil, fmt.Errorf("expected multipart file or JSON body: %w", err)
	}
	if body.DocID != "" {
		docID = body.DocID
	}
	mt := body.MediaType
	if mt == "" {
		mt = mediaTypeFromExt(body.Source)
	}
	src := body.Source
	if src == "" {
		src = "document"
	}
	return docID, mt, src, []byte(body.Content), nil
}

// mediaTypeFromExt guesses a MIME type from a filename extension.
func mediaTypeFromExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".csv":
		return "text/csv"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/x-yaml"
	default:
		return "text/plain"
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
