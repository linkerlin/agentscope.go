package rerank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/memory/vector"
)

type fakeEmbedding struct {
	dim int
}

func (f *fakeEmbedding) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, f.dim)
	for i, c := range []byte(text) {
		vec[i%f.dim] += float32(c)
	}
	return vec, nil
}

func (f *fakeEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := f.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func TestNoopReranker(t *testing.T) {
	r := NoopReranker{}
	docs := []Document{
		{ID: "a", Content: "foo", Score: 0.1},
		{ID: "b", Content: "bar", Score: 0.5},
	}
	got, err := r.Rerank(context.Background(), "q", docs, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].ID != "a" {
		t.Fatalf("expected id a, got %s", got[0].ID)
	}
	if got[0].RelevanceScore != 0.1 {
		t.Fatalf("expected relevance score 0.1, got %f", got[0].RelevanceScore)
	}
}

func TestLocalReranker(t *testing.T) {
	embed := &fakeEmbedding{dim: 8}
	r := NewLocalReranker(embed)
	docs := []Document{
		{ID: "a", Content: "quick brown fox"},
		{ID: "b", Content: "unrelated content about space"},
		{ID: "c", Content: "another irrelevant document"},
	}

	got, err := r.Rerank(context.Background(), "quick brown fox", docs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].ID != "a" {
		t.Fatalf("expected top result id a, got %s", got[0].ID)
	}
	if got[0].RelevanceScore <= 0 || got[0].RelevanceScore > 1+1e-9 {
		t.Fatalf("cosine score out of bounds: %f", got[0].RelevanceScore)
	}
	if got[0].RelevanceScore < got[1].RelevanceScore {
		t.Fatalf("expected descending scores, got %f then %f", got[0].RelevanceScore, got[1].RelevanceScore)
	}
}

func TestLocalRerankerEmpty(t *testing.T) {
	embed := &fakeEmbedding{dim: 4}
	r := NewLocalReranker(embed)
	got, err := r.Rerank(context.Background(), "q", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty results, got %d", len(got))
	}
}

func TestCohereReranker(t *testing.T) {
	docs := []Document{
		{ID: "a", Content: "foo"},
		{ID: "b", Content: "bar"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("missing authorization header")
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"index": 1, "relevance_score": 0.9},
				{"index": 0, "relevance_score": 0.2},
			},
		})
	}))
	defer server.Close()

	r := NewCohereReranker("key", "rerank-foo")
	r.baseURL = server.URL
	got, err := r.Rerank(context.Background(), "q", docs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].ID != "b" || got[0].RelevanceScore != 0.9 {
		t.Fatalf("unexpected top result: %+v", got[0])
	}
	if got[1].ID != "a" || got[1].RelevanceScore != 0.2 {
		t.Fatalf("unexpected second result: %+v", got[1])
	}
}

func TestCohereRerankerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid api key"))
	}))
	defer server.Close()

	r := NewCohereReranker("bad", "m")
	r.baseURL = server.URL
	_, err := r.Rerank(context.Background(), "q", []Document{{ID: "a", Content: "x"}}, 1)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestJinaReranker(t *testing.T) {
	docs := []Document{
		{ID: "a", Content: "foo"},
		{ID: "b", Content: "bar"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("missing authorization header")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"index": 0, "relevance_score": 0.8, "document": map[string]string{"text": "foo"}},
				{"index": 1, "relevance_score": 0.3, "document": map[string]string{"text": "bar"}},
			},
		})
	}))
	defer server.Close()

	r := NewJinaReranker("key", "jina-reranker")
	r.baseURL = server.URL
	got, err := r.Rerank(context.Background(), "q", docs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("unexpected results: %+v", got)
	}
	if got[0].RelevanceScore < got[1].RelevanceScore {
		t.Fatal("expected descending scores")
	}
}

func TestJinaRerankerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	r := NewJinaReranker("bad", "m")
	r.baseURL = server.URL
	_, err := r.Rerank(context.Background(), "q", []Document{{ID: "a", Content: "x"}}, 1)
	if err == nil {
		t.Fatal("expected error for 400")
	}
}

func TestLocalRerankerBadDimensions(t *testing.T) {
	embed := &fakeEmbedding{dim: 4}
	r := NewLocalReranker(embed)
	// The first document embedding will have dim 4; the second query embedding also dim 4.
	// Use mismatched input lengths via different fakeEmbedding dims is not possible with same model,
	// so verify graceful handling when embedding returns zero vector.
	got, err := r.Rerank(context.Background(), "", []Document{{ID: "a", Content: "x"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].RelevanceScore != 0 {
		t.Fatalf("expected zero score for zero query vector, got %f", got[0].RelevanceScore)
	}
}

var _ vector.EmbeddingModel = (*fakeEmbedding)(nil)
