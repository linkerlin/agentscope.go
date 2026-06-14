package vector

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fixedEmbed struct {
	dim int
}

func (e fixedEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, e.dim)
	for i := range v {
		v[i] = float32((i + 1) * (len(text)%10 + 1))
	}
	return v, nil
}

func (e fixedEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, _ := e.Embed(ctx, t)
		out[i] = v
	}
	return out, nil
}

// --- Qdrant (real REST implementation; integration tests in integration_test.go) ---

func TestQdrantVectorStoreNilEmbed(t *testing.T) {
	_, err := NewQdrantVectorStore("http://localhost:6334", "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

// --- Elasticsearch stub ---

func TestElasticsearchVectorStoreStub(t *testing.T) {
	e := fixedEmbed{dim: 4}
	s, err := NewElasticsearchVectorStore([]string{"http://localhost:9200"}, "test", 4, e)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(context.Background(), nil); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	nodes, err := s.Search(context.Background(), "", RetrieveOptions{})
	if err != ErrNotImplemented || nodes != nil {
		t.Fatalf("expected ErrNotImplemented and nil nodes, got %v %v", nodes, err)
	}
}

func TestElasticsearchVectorStoreNilEmbed(t *testing.T) {
	_, err := NewElasticsearchVectorStore([]string{"http://localhost:9200"}, "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

// --- PGVector stub ---

func TestPgvectorVectorStoreStub(t *testing.T) {
	e := fixedEmbed{dim: 4}
	s, err := NewPgvectorVectorStore("postgres://localhost/test", "test", 4, e)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Insert(context.Background(), nil)
	_, _ = s.Search(context.Background(), "", RetrieveOptions{})
}

func TestPgvectorVectorStoreNilEmbed(t *testing.T) {
	_, err := NewPgvectorVectorStore("postgres://localhost/test", "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

// --- Chroma (real implementation with network — only test nil embed) ---

func TestChromaVectorStoreNilEmbed(t *testing.T) {
	_, err := NewChromaVectorStore("http://localhost:8000", "test", 4, nil)
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}

// --- RawVectorStore stub ---

func TestRawVectorStoreStub(t *testing.T) {
	s := NewRawVectorStore(nil)
	_ = s.Insert(context.Background(), nil)
	nodes, _ := s.Search(context.Background(), "", RetrieveOptions{})
	if nodes != nil {
		t.Fatal("stub Search should return nil")
	}
	node, _ := s.Get(context.Background(), "x")
	if node != nil {
		t.Fatal("stub Get should return nil")
	}
	_ = s.Update(context.Background(), nil)
	_ = s.Delete(context.Background(), "x")
	_ = s.DeleteAll(context.Background())
}

// --- Snapshot stub ---

func TestSnapshotStoreStub(t *testing.T) {
	e := fixedEmbed{dim: 4}
	s := NewLocalVectorStore(e)
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	if err := s.WriteSnapshot(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("snapshot file not written")
	}

	s2 := NewLocalVectorStore(e)
	if err := s2.ReadSnapshot(path); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotStoreRoundTrip(t *testing.T) {
	e := fixedEmbed{dim: 4}
	s := NewLocalVectorStore(e)
	ctx := context.Background()

	n := &MemoryNode{
		MemoryID:     "test1",
		MemoryType:   MemoryTypePersonal,
		MemoryTarget: "bob",
		Content:      "hello world",
		Vector:       []float32{1, 2, 3, 4},
	}
	if err := s.Insert(ctx, []*MemoryNode{n}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := s.WriteSnapshot(path); err != nil {
		t.Fatal(err)
	}

	// Verify JSON contains the node.
	data, _ := os.ReadFile(path)
	var nodes map[string]*MemoryNode
	_ = json.Unmarshal(data, &nodes)
	if _, ok := nodes["test1"]; !ok {
		t.Fatal("snapshot should contain test1")
	}

	s2 := NewLocalVectorStore(e)
	if err := s2.ReadSnapshot(path); err != nil {
		t.Fatal(err)
	}
	node, err := s2.Get(ctx, "test1")
	if err != nil || node == nil {
		t.Fatalf("round-trip: %v %v", node, err)
	}
}

func TestSnapshotStoreNilEmbed(t *testing.T) {
	err := NewLocalVectorStore(nil).Insert(context.Background(), []*MemoryNode{{Content: "x"}})
	if err != ErrEmbeddingRequired {
		t.Fatalf("expected ErrEmbeddingRequired, got %v", err)
	}
}
