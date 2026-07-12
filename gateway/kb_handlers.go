package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/index"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

// KBService bundles the components needed to serve the knowledge-base HTTP
// API: a KB manager, blob store, parser registry, chunker, and a pipeline
// worker. Attach it to a Server via WithKBService, then call RegisterKBRoutes.
type KBService struct {
	Manager *kb.KBManager
	Blob    blob.BlobStore
	Parsers *parser.Registry
	Chunker chunker.Chunker
	Worker  *index.Worker
	// NewParserRegistry returns a fresh registry when an upload needs custom
	// parser routing (e.g. per-tenant parsers). Defaults to Parsers if nil.
}

// NewKBService builds a KBService from the core components, auto-creating the
// pipeline Worker.
func NewKBService(mgr *kb.KBManager, bs blob.BlobStore, parsers *parser.Registry, ch chunker.Chunker) *KBService {
	svc := &KBService{Manager: mgr, Blob: bs, Parsers: parsers, Chunker: ch}
	svc.Worker = &index.Worker{Blob: bs, Parsers: parsers, Chunker: ch, Manager: mgr}
	return svc
}

// WithKBService attaches a knowledge-base service for KB HTTP endpoints.
func (s *Server) WithKBService(svc *KBService) *Server {
	s.kbService = svc
	return s
}

// RegisterKBRoutes registers the knowledge-base CRUD + upload + search routes.
// No-op if no KBService is attached.
func (s *Server) RegisterKBRoutes() {
	svc := s.kbService
	if svc == nil {
		return
	}
	mux := s.mux
	mux.HandleFunc("GET /api/v1/knowledge-bases", s.requireAuth(s.handleListKnowledgeBases))
	mux.HandleFunc("POST /api/v1/knowledge-bases", s.requireAuth(s.handleCreateKnowledgeBase))
	mux.HandleFunc("GET /api/v1/knowledge-bases/{id}", s.requireAuth(s.handleGetKnowledgeBase))
	mux.HandleFunc("DELETE /api/v1/knowledge-bases/{id}", s.requireAuth(s.handleDeleteKnowledgeBase))
	mux.HandleFunc("POST /api/v1/knowledge-bases/{id}/documents", s.requireAuth(s.handleUploadDocument))
	mux.HandleFunc("GET /api/v1/knowledge-bases/{id}/documents", s.requireAuth(s.handleListDocuments))
	mux.HandleFunc("DELETE /api/v1/knowledge-bases/{id}/documents/{doc_id}", s.requireAuth(s.handleDeleteDocument))
	mux.HandleFunc("POST /api/v1/knowledge-bases/{id}/search", s.requireAuth(s.handleSearchKnowledgeBase))
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

func (s *Server) handleListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	specs := s.kbService.Manager.List(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"knowledge_bases": specs})
}

func (s *Server) handleCreateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	var req createKBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	if err := s.kbService.Manager.Create(r.Context(), spec); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (s *Server) handleGetKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	h, err := s.kbService.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	docs, _ := h.ListDocuments(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        h.Name,
		"description": h.Description,
		"documents":   docs,
	})
}

func (s *Server) handleDeleteKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	if err := s.kbService.Manager.Delete(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	h, err := s.kbService.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	docID, mediaType, source, data, err := readUpload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	blobKey := fmt.Sprintf("%s/%s", name, docID)
	uri, err := s.kbService.Blob.Put(ctx, blobKey, strings.NewReader(string(data)))
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
	s.kbService.Worker.OnStatus = func(s2 index.Status) { status = s2 }
	if err := s.kbService.Worker.Process(ctx, task); err != nil {
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
		"document": h, //nolint — keep handle in response for traceability
	})
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	h, err := s.kbService.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	docs, err := h.ListDocuments(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	docID := r.PathValue("doc_id")
	h, err := s.kbService.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := h.DeleteDocument(r.Context(), docID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSearchKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	h, err := s.kbService.Manager.Get(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var req searchKBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	results, err := h.Search(r.Context(), req.Query, req.TopK)
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

// readUpload extracts document bytes from either a multipart/form-data upload
// or a JSON body. Returns (docID, mediaType, source, data, err).
func readUpload(r *http.Request) (string, string, string, []byte, error) {
	docID := generateID("doc")
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
