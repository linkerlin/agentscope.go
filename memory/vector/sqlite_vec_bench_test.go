package vector

import (
	"context"
	"fmt"
	"testing"
)

type benchEmbedder struct {
	dim int
}

func (m *benchEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return deterministicVec(text, m.dim), nil
}

func (m *benchEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = deterministicVec(t, m.dim)
	}
	return out, nil
}

func deterministicVec(text string, dim int) []float32 {
	v := make([]float32, dim)
	for i := 0; i < dim; i++ {
		v[i] = float32(len(text)*17 + i*31)
	}
	return v
}

func BenchmarkSQLiteVec_Insert(b *testing.B) {
	store, err := NewSQLiteVecStore(":memory:", 64, &benchEmbedder{dim: 64})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	nodes := make([]*MemoryNode, b.N)
	for i := 0; i < b.N; i++ {
		nodes[i] = &MemoryNode{
			MemoryID:   fmt.Sprintf("bench-%d", i),
			Content:    fmt.Sprintf("content %d", i),
			MemoryType: MemoryTypePersonal,
		}
	}

	b.ResetTimer()
	_ = store.Insert(context.Background(), nodes)
}

func BenchmarkSQLiteVec_Search(b *testing.B) {
	store, err := NewSQLiteVecStore(":memory:", 64, &benchEmbedder{dim: 64})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 500; i++ {
		_ = store.Insert(context.Background(), []*MemoryNode{{
			MemoryID:   fmt.Sprintf("vec-%d", i),
			Content:    fmt.Sprintf("document content %d", i),
			MemoryType: MemoryTypePersonal,
		}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Search(context.Background(), "query content", RetrieveOptions{TopK: 10})
	}
}

func BenchmarkSQLiteVec_Get(b *testing.B) {
	store, err := NewSQLiteVecStore(":memory:", 32, &benchEmbedder{dim: 32})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	_ = store.Insert(context.Background(), []*MemoryNode{{
		MemoryID:   "get-1",
		Content:    "test",
		MemoryType: MemoryTypePersonal,
	}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(context.Background(), "get-1")
	}
}

func BenchmarkSQLiteVec_Delete(b *testing.B) {
	store, err := NewSQLiteVecStore(":memory:", 32, &benchEmbedder{dim: 32})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	_ = store.Insert(context.Background(), []*MemoryNode{{
		MemoryID:   "del-1",
		Content:    "test",
		MemoryType: MemoryTypePersonal,
	}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Delete(context.Background(), "del-1")
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float32, 384)
	v := make([]float32, 384)
	for i := range a {
		a[i] = float32(i) * 0.01
		v[i] = float32(i) * 0.02
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(a, v)
	}
}

func BenchmarkNormalizeVector(b *testing.B) {
	v := make([]float32, 384)
	for i := range v {
		v[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalizeVector(v)
	}
}
