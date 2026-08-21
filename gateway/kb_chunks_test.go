package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/rag/kb"
)

// metaIndex reads a chunk_index that may be int (direct) or float64 (JSON
// round-trip) depending on how the response was decoded.
func metaIndex(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return -1
}

// uploadDoc uploads one text document into a KB and fails the test on error.
func uploadDoc(t *testing.T, srv *Server, kbName, docID, content string) {
	t.Helper()
	w := doRequest(t, srv, "POST", "/api/v1/knowledge-bases/"+kbName+"/documents",
		map[string]any{"doc_id": docID, "content": content, "media_type": "text/plain", "source": docID + ".txt"})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload %s: got %d %s", docID, w.Code, w.Body.String())
	}
}

func TestKB_ListDocChunks(t *testing.T) {
	srv := newTestKBServer(t)
	doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "docs", EmbedderID: "stub"})
	// Long enough for the approx-token chunker to split into several chunks.
	content := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 120)
	uploadDoc(t, srv, "docs", "d1", content)

	w := doRequest(t, srv, "GET", "/api/v1/knowledge-bases/docs/documents/d1/chunks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("chunks: got %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Chunks []kb.Record `json:"chunks"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(resp.Chunks))
	}
	// Ordered by chunk index, no vectors, blob_uri tagged for raw preview.
	for i, c := range resp.Chunks {
		if c.Vector != nil {
			t.Fatalf("chunk %d: vector must be stripped", i)
		}
		if idx := metaIndex(c.Metadata["chunk_index"]); idx != i {
			t.Fatalf("chunk %d: metadata chunk_index = %v", i, c.Metadata["chunk_index"])
		}
		if uri, _ := c.Metadata["blob_uri"].(string); !strings.HasPrefix(uri, "local://") {
			t.Fatalf("chunk %d: missing blob_uri, metadata=%v", i, c.Metadata)
		}
	}

	// Unknown doc -> empty list (200), unknown KB -> 404.
	w = doRequest(t, srv, "GET", "/api/v1/knowledge-bases/docs/documents/nope/chunks", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"chunks":[]`) {
		t.Fatalf("unknown doc: got %d %s", w.Code, w.Body.String())
	}
	w = doRequest(t, srv, "GET", "/api/v1/knowledge-bases/missing/documents/d1/chunks", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown kb: got %d", w.Code)
	}
}

func TestKB_RawDocument(t *testing.T) {
	srv := newTestKBServer(t)
	doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "docs", EmbedderID: "stub"})
	content := "raw bytes of the document"
	uploadDoc(t, srv, "docs", "d1", content)

	w := doRequest(t, srv, "GET", "/api/v1/knowledge-bases/docs/documents/d1/raw", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("raw: got %d %s", w.Code, w.Body.String())
	}
	if w.Body.String() != content {
		t.Fatalf("raw content mismatch: %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("content type: %s", ct)
	}

	// Unknown doc -> 404.
	w = doRequest(t, srv, "GET", "/api/v1/knowledge-bases/docs/documents/nope/raw", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown doc raw: got %d", w.Code)
	}
}

func TestKB_ListEnrichedWithCounts(t *testing.T) {
	srv := newTestKBServer(t)
	doRequest(t, srv, "POST", "/api/v1/knowledge-bases", createKBRequest{Name: "docs", EmbedderID: "stub"})
	content := strings.Repeat("alpha beta gamma delta. ", 100)
	uploadDoc(t, srv, "docs", "d1", content)
	uploadDoc(t, srv, "docs", "d2", content)

	w := doRequest(t, srv, "GET", "/api/v1/knowledge-bases", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d", w.Code)
	}
	var resp struct {
		KnowledgeBases []struct {
			kb.Spec
			Documents int `json:"documents"`
			Chunks    int `json:"chunks"`
		} `json:"knowledge_bases"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.KnowledgeBases) != 1 {
		t.Fatalf("expected 1 kb, got %d", len(resp.KnowledgeBases))
	}
	got := resp.KnowledgeBases[0]
	if got.Documents != 2 || got.Chunks == 0 {
		t.Fatalf("enriched counts wrong: documents=%d chunks=%d", got.Documents, got.Chunks)
	}
}
