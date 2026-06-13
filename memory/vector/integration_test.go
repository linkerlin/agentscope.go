//go:build integration

package vector

import (
	"context"
	"os"
	"testing"
	"time"
)

// mockEmbedder is a tiny deterministic embedder for integration tests.
type mockEmbedder struct{ dim int }

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, m.dim)
	for i := range v {
		v[i] = float32(i) * 0.01
	}
	return v, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		v, _ := m.Embed(ctx, texts[i])
		out[i] = v
	}
	return out, nil
}

func skipIfNoDocker(t *testing.T) {
	if os.Getenv("VECTOR_STORE_INTEGRATION") != "1" {
		t.Skip("set VECTOR_STORE_INTEGRATION=1 to run vector store integration tests")
	}
}

func TestChromaVectorStore_Integration(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()
	embed := &mockEmbedder{dim: 8}

	s, err := NewChromaVectorStore("http://localhost:8000", "test_agentscope", 8, embed)
	if err != nil {
		t.Fatalf("new chroma: %v", err)
	}
	_ = s.DeleteAll(ctx)

	if err := s.Insert(ctx, []*MemoryNode{
		{MemoryID: "c1", Content: "hello world", MemoryType: MemoryTypeHistory},
		{MemoryID: "c2", Content: "go programming", MemoryType: MemoryTypeHistory},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	nodes, err := s.Search(ctx, "hello", RetrieveOptions{TopK: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected search results")
	}

	got, err := s.Get(ctx, "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "hello world" {
		t.Fatalf("unexpected content: %s", got.Content)
	}

	if err := s.Delete(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestQdrantVectorStore_Integration(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()
	embed := &mockEmbedder{dim: 8}

	s, err := NewQdrantVectorStore("http://localhost:6333", "test_agentscope", 8, embed)
	if err != nil {
		t.Fatalf("new qdrant: %v", err)
	}
	_ = s.DeleteAll(ctx)

	if err := s.Insert(ctx, []*MemoryNode{
		{MemoryID: "q1", Content: "hello world", MemoryType: MemoryTypeHistory},
		{MemoryID: "q2", Content: "go programming", MemoryType: MemoryTypeHistory},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	nodes, err := s.Search(ctx, "hello", RetrieveOptions{TopK: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected search results")
	}

	got, err := s.Get(ctx, "q1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "hello world" {
		t.Fatalf("unexpected content: %s", got.Content)
	}

	if err := s.Delete(ctx, "q1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMilvusVectorStore_Integration(t *testing.T) {
	skipIfNoDocker(t)
	ctx := context.Background()
	embed := &mockEmbedder{dim: 8}

	s, err := NewMilvusVectorStore("http://localhost:19530", "test_agentscope", 8, embed)
	if err != nil {
		t.Fatalf("new milvus: %v", err)
	}
	_ = s.DeleteAll(ctx)

	if err := s.Insert(ctx, []*MemoryNode{
		{MemoryID: "m1", Content: "hello world", MemoryType: MemoryTypeHistory},
		{MemoryID: "m2", Content: "go programming", MemoryType: MemoryTypeHistory},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	nodes, err := s.Search(ctx, "hello", RetrieveOptions{TopK: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected search results")
	}

	got, err := s.Get(ctx, "m1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "hello world" {
		t.Fatalf("unexpected content: %s", got.Content)
	}

	if err := s.Delete(ctx, "m1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
