package vector

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// stressEmbedder generates deterministic vectors for stress testing.
type stressEmbedder struct {
	dim int
}

func (m *stressEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return deterministicVec(text, m.dim), nil
}

func (m *stressEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = deterministicVec(t, m.dim)
	}
	return out, nil
}

// BenchmarkLocalVectorStore_SearchLargeDataset benchmarks search on a large in-memory store.
func BenchmarkLocalVectorStore_SearchLargeDataset(b *testing.B) {
	for _, size := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			store := NewLocalVectorStore(&stressEmbedder{dim: 128})
			nodes := make([]*MemoryNode, size)
			for i := 0; i < size; i++ {
				nodes[i] = &MemoryNode{
					MemoryID:   fmt.Sprintf("stress-%d", i),
					Content:    fmt.Sprintf("document about topic %d with various keywords", i),
					MemoryType: MemoryTypePersonal,
				}
			}
			_ = store.Insert(context.Background(), nodes)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = store.Search(context.Background(),
					fmt.Sprintf("query about topic %d", i%size),
					RetrieveOptions{TopK: 10})
			}
		})
	}
}

// BenchmarkLocalVectorStore_InsertBatch benchmarks batch insertion.
func BenchmarkLocalVectorStore_InsertBatch(b *testing.B) {
	for _, batchSize := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				store := NewLocalVectorStore(&stressEmbedder{dim: 64})
				nodes := make([]*MemoryNode, batchSize)
				for j := 0; j < batchSize; j++ {
					nodes[j] = &MemoryNode{
						MemoryID:   fmt.Sprintf("batch-%d-%d", i, j),
						Content:    fmt.Sprintf("content item %d in batch %d", j, i),
						MemoryType: MemoryTypePersonal,
					}
				}
				b.StartTimer()
				_ = store.Insert(context.Background(), nodes)
			}
		})
	}
}

// BenchmarkLocalVectorStore_GetConcurrent measures concurrent Get operations.
func BenchmarkLocalVectorStore_GetConcurrent(b *testing.B) {
	store := NewLocalVectorStore(&stressEmbedder{dim: 64})
	nodes := make([]*MemoryNode, 1000)
	for i := 0; i < 1000; i++ {
		nodes[i] = &MemoryNode{
			MemoryID:   fmt.Sprintf("get-%d", i),
			Content:    fmt.Sprintf("content %d", i),
			MemoryType: MemoryTypePersonal,
		}
	}
	_ = store.Insert(context.Background(), nodes)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			_, _ = store.Get(context.Background(), fmt.Sprintf("get-%d", i%1000))
			i++
		}
	})
}

// BenchmarkLocalVectorStore_SearchConcurrent measures concurrent Search operations.
func BenchmarkLocalVectorStore_SearchConcurrent(b *testing.B) {
	store := NewLocalVectorStore(&stressEmbedder{dim: 64})
	nodes := make([]*MemoryNode, 500)
	for i := 0; i < 500; i++ {
		nodes[i] = &MemoryNode{
			MemoryID:   fmt.Sprintf("search-%d", i),
			Content:    fmt.Sprintf("document content %d", i),
			MemoryType: MemoryTypePersonal,
		}
	}
	_ = store.Insert(context.Background(), nodes)

	for _, concurrency := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("concurrency=%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				var i int
				for pb.Next() {
					_, _ = store.Search(context.Background(),
						fmt.Sprintf("query %d", i%500),
						RetrieveOptions{TopK: 10})
					i++
				}
			})
		})
	}
}

// BenchmarkLocalVectorStore_DeleteConcurrent measures concurrent Delete operations.
func BenchmarkLocalVectorStore_DeleteConcurrent(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store := NewLocalVectorStore(&stressEmbedder{dim: 32})
		nodes := make([]*MemoryNode, 200)
		for j := 0; j < 200; j++ {
			nodes[j] = &MemoryNode{
				MemoryID:   fmt.Sprintf("del-%d-%d", i, j),
				Content:    fmt.Sprintf("content %d", j),
				MemoryType: MemoryTypePersonal,
			}
		}
		_ = store.Insert(context.Background(), nodes)
		b.StartTimer()

		var wg sync.WaitGroup
		for j := 0; j < 200; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_ = store.Delete(context.Background(), fmt.Sprintf("del-%d-%d", i, idx))
			}(j)
		}
		wg.Wait()
	}
}

// BenchmarkLocalVectorStore_MixedWorkload simulates a realistic read-heavy workload
// (80% search, 15% get, 5% insert).
func BenchmarkLocalVectorStore_MixedWorkload(b *testing.B) {
	store := NewLocalVectorStore(&stressEmbedder{dim: 64})
	for i := 0; i < 500; i++ {
		_ = store.Insert(context.Background(), []*MemoryNode{{
			MemoryID:   fmt.Sprintf("mixed-%d", i),
			Content:    fmt.Sprintf("document %d about AI and technology", i),
			MemoryType: MemoryTypePersonal,
		}})
	}

	var insertIdx int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var op int
		for pb.Next() {
			op++
			switch op % 20 {
			case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15: // 80% search
				_, _ = store.Search(context.Background(), "AI technology", RetrieveOptions{TopK: 5})
			case 16, 17, 18: // 15% get
				_, _ = store.Get(context.Background(), fmt.Sprintf("mixed-%d", op%500))
			default: // 5% insert
				idx := insertIdx + 1
				_ = store.Insert(context.Background(), []*MemoryNode{{
					MemoryID:   fmt.Sprintf("new-%d", idx),
					Content:    fmt.Sprintf("new document %d", idx),
					MemoryType: MemoryTypePersonal,
				}})
			}
		}
	})
}
