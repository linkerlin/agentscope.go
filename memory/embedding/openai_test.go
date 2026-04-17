package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	goopenai "github.com/sashabaranov/go-openai"
)

func TestOpenAIEmbedder_Dimensions(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	e := NewOpenAIEmbedder(apiKey, "text-embedding-3-small")
	ctx := context.Background()

	vec, err := e.Embed(ctx, "hello world")
	if err != nil {
		if isAuthError(err) {
			t.Skip("invalid OPENAI_API_KEY, skipping live test")
		}
		t.Fatalf("embed failed: %v", err)
	}
	if len(vec) == 0 {
		t.Fatal("expected non-empty vector")
	}

	vecs, err := e.EmbedBatch(ctx, []string{"hello", "world"})
	if err != nil {
		if isAuthError(err) {
			t.Skip("invalid OPENAI_API_KEY, skipping live test")
		}
		t.Fatalf("embed batch failed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) == 0 || len(vecs[1]) == 0 {
		t.Fatal("expected non-empty vectors in batch")
	}
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "Incorrect API key")
}

func TestOpenAIEmbedder_DefaultModel(t *testing.T) {
	e := NewOpenAIEmbedder("fake-key", "")
	if e.modelName == "" {
		t.Fatal("expected default model name")
	}
}

func TestLocalEmbedder_NotImplemented(t *testing.T) {
	e := NewLocalEmbedder("http://localhost:11434", "nomic-embed-text", 768)
	ctx := context.Background()

	_, err := e.Embed(ctx, "test")
	if err == nil {
		t.Fatal("expected error for unimplemented local embedder")
	}

	_, err = e.EmbedBatch(ctx, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for unimplemented local embedder batch")
	}
}

func TestOpenAIEmbedderWithBaseURL(t *testing.T) {
	e := NewOpenAIEmbedderWithBaseURL("key", "http://localhost:8080", "model-x")
	if e.modelName != "model-x" {
		t.Fatalf("expected model-x, got %s", e.modelName)
	}
	e2 := NewOpenAIEmbedderWithBaseURL("key", "", "")
	if e2.modelName == "" {
		t.Fatal("expected default model name")
	}
}

func TestOpenAIEmbedder_Embed_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			http.NotFound(w, r)
			return
		}
		resp := goopenai.EmbeddingResponse{
			Object: "list",
			Data: []goopenai.Embedding{
				{Object: "embedding", Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
			},
			Model: "text-embedding-3-small",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := goopenai.DefaultConfig("fake-key")
	cfg.BaseURL = ts.URL
	e := &OpenAIEmbedder{client: goopenai.NewClientWithConfig(cfg), modelName: "text-embedding-3-small"}

	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 3 {
		t.Fatalf("expected 3 dims, got %d", len(vec))
	}
}

func TestOpenAIEmbedder_Embed_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := goopenai.EmbeddingResponse{Object: "list", Data: []goopenai.Embedding{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := goopenai.DefaultConfig("fake-key")
	cfg.BaseURL = ts.URL
	e := &OpenAIEmbedder{client: goopenai.NewClientWithConfig(cfg), modelName: "text-embedding-3-small"}

	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestOpenAIEmbedder_EmbedBatch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := goopenai.EmbeddingResponse{
			Object: "list",
			Data: []goopenai.Embedding{
				{Object: "embedding", Embedding: []float32{0.1}, Index: 0},
				{Object: "embedding", Embedding: []float32{0.2}, Index: 1},
			},
			Model: "text-embedding-3-small",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := goopenai.DefaultConfig("fake-key")
	cfg.BaseURL = ts.URL
	e := &OpenAIEmbedder{client: goopenai.NewClientWithConfig(cfg), modelName: "text-embedding-3-small"}

	vecs, err := e.EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
}

func TestOpenAIEmbedder_EmbedBatch_EmptyInput(t *testing.T) {
	e := NewOpenAIEmbedder("fake-key", "text-embedding-3-small")
	vecs, err := e.EmbedBatch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if vecs != nil {
		t.Fatal("expected nil for empty input")
	}
}

func TestOpenAIEmbedder_EmbedBatch_Mismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := goopenai.EmbeddingResponse{
			Object: "list",
			Data:   []goopenai.Embedding{{Object: "embedding", Embedding: []float32{0.1}, Index: 0}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := goopenai.DefaultConfig("fake-key")
	cfg.BaseURL = ts.URL
	e := &OpenAIEmbedder{client: goopenai.NewClientWithConfig(cfg), modelName: "text-embedding-3-small"}

	_, err := e.EmbedBatch(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for result count mismatch")
	}
}
