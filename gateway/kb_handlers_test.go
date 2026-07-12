package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

type stubKHEmbedder struct{}

func (stubKHEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// deterministic 4-dim vector from text length
	v := make([]float32, 4)
	for i := range v {
		v[i] = float32(len(text) % (i*7 + 3))
	}
	return v, nil
}

func newTestKBServer(t *testing.T) *Server {
	t.Helper()
	bs, err := blob.NewLocalBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry(parser.NewTextParser(), parser.NewPPTXParser())
	ch := chunker.NewApproxTokenChunker()
	mgr := kb.NewCollectionPerKBManager(kb.NewInMemoryVectorStore(), func(string) (kb.Embedder, error) {
		return stubKHEmbedder{}, nil
	})
	svc := NewKBService(mgr, bs, reg, ch)
	srv := NewServer(&mockAgent{})
	srv.WithKBService(svc)
	srv.RegisterKBRoutes()
	return srv
}

func doRequest(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		if raw, ok := body.([]byte); ok {
			r = bytes.NewReader(raw)
		} else {
			data, _ := json.Marshal(body)
			r = bytes.NewReader(data)
		}
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestKB_CreateListDelete(t *testing.T) {
	srv := newTestKBServer(t)

	// Create
	w := doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{
		Name: "docs", Description: "test", EmbedderID: "stub",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d %s", w.Code, w.Body.String())
	}

	// Duplicate -> 409
	w = doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "docs"})
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate: got %d", w.Code)
	}

	// List
	w = doRequest(t, srv, "GET", "/api/v1/knowledge-bases", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d", w.Code)
	}
	var listResp struct {
		KnowledgeBases []kb.Spec `json:"knowledge_bases"`
	}
	json.NewDecoder(w.Body).Decode(&listResp)
	if len(listResp.KnowledgeBases) != 1 || listResp.KnowledgeBases[0].Name != "docs" {
		t.Fatalf("unexpected list: %+v", listResp)
	}

	// Delete
	w = doRequest(t, srv, "DELETE", "/api/v1/knowledge-bases/docs", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d", w.Code)
	}
	// Delete missing -> 404
	w = doRequest(t, srv, "DELETE", "/api/v1/knowledge-bases/docs", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete missing: got %d", w.Code)
	}
}

func TestKB_UploadSearchDeleteDoc(t *testing.T) {
	srv := newTestKBServer(t)
	doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "kb1", EmbedderID: "s"})

	// Upload via JSON
	w := doRequest(t, srv, "POST", "/api/v1/knowledge-bases/kb1/documents", map[string]any{
		"content":    "The PTO policy grants 15 days per year.",
		"media_type": "text/plain",
		"source":     "pto.txt",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: got %d %s", w.Code, w.Body.String())
	}
	var upResp struct {
		DocID  string `json:"doc_id"`
		Chunks int    `json:"chunks"`
	}
	json.NewDecoder(w.Body).Decode(&upResp)
	if upResp.Chunks == 0 {
		t.Fatal("expected chunks indexed")
	}

	// Search
	w = doRequest(t, srv, "POST", "/api/v1/knowledge-bases/kb1/search", searchKBRequest{Query: "PTO policy"})
	if w.Code != http.StatusOK {
		t.Fatalf("search: got %d %s", w.Code, w.Body.String())
	}
	var sResp struct {
		Results []searchResultItem `json:"results"`
	}
	json.NewDecoder(w.Body).Decode(&sResp)
	if len(sResp.Results) == 0 {
		t.Fatal("expected search results")
	}
	if !strings.Contains(sResp.Results[0].Text, "PTO") {
		t.Fatalf("unexpected result: %q", sResp.Results[0].Text)
	}

	// List docs
	w = doRequest(t, srv, "GET", "/api/v1/knowledge-bases/kb1/documents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list docs: got %d", w.Code)
	}

	// Delete doc
	w = doRequest(t, srv, "DELETE", "/api/v1/knowledge-bases/kb1/documents/"+upResp.DocID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete doc: got %d", w.Code)
	}
}

func TestKB_UploadMultipart(t *testing.T) {
	srv := newTestKBServer(t)
	doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "kb2", EmbedderID: "s"})

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, _ := mw.CreateFormFile("file", "note.md")
	fw.Write([]byte("# Notes\nSome important notes here."))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/knowledge-bases/kb2/documents", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("multipart upload: got %d %s", w.Code, w.Body.String())
	}
}

func TestKB_SearchMissingKB(t *testing.T) {
	srv := newTestKBServer(t)
	w := doRequest(t, srv, "POST", "/api/v1/knowledge-bases/ghost/search", searchKBRequest{Query: "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestKB_SearchEmptyQuery(t *testing.T) {
	srv := newTestKBServer(t)
	doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "kb3", EmbedderID: "s"})
	w := doRequest(t, srv, "POST", "/api/v1/knowledge-bases/kb3/search", searchKBRequest{Query: ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty query, got %d", w.Code)
	}
}

func TestMediaTypeFromExt(t *testing.T) {
	cases := map[string]string{
		"a.pdf":  "application/pdf",
		"a.pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"a.md":   "text/markdown",
		"a.csv":  "text/csv",
		"a.xyz":  "text/plain",
	}
	for name, want := range cases {
		if got := mediaTypeFromExt(name); got != want {
			t.Errorf("%s: got %s want %s", name, got, want)
		}
	}
}
