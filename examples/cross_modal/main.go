// examples/cross_modal/main.go
//
// Demo: Cross-modal memory search — text query for image results.
//
// This demo shows how to create a CrossModalSearcher and retrieve mixed
// results (text + image) using a text query. Mock embedders are used so
// no real services are required.
//
// How to run:
//   cd examples/cross_modal && go run main.go

package main

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/memory"
)

// mockEmbedder is a minimal text embedder for demo purposes.
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{float32(len(text)), 0.2, 0.3, 0.4}, nil
}
func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for _, t := range texts {
		v, _ := m.Embed(ctx, t)
		out = append(out, v)
	}
	return out, nil
}

// mockImageEmbedder is a minimal image embedder for demo purposes.
type mockImageEmbedder struct{}

func (m *mockImageEmbedder) EmbedImage(ctx context.Context, url, base64 string) ([]float32, error) {
	return []float32{0.4, 0.3, 0.2, float32(len(url))}, nil
}
func (m *mockImageEmbedder) EmbedImageBatch(ctx context.Context, urls []string) ([][]float32, error) {
	var out [][]float32
	for _, u := range urls {
		v, _ := m.EmbedImage(ctx, u, "")
		out = append(out, v)
	}
	return out, nil
}
func (m *mockImageEmbedder) Dimension() int { return 4 }

func main() {
	ctx := context.Background()

	// 1. Build a local vector store and seed it with text and image memories.
	embedder := &mockEmbedder{}
	store := memory.NewLocalVectorStore(embedder)

	textNode := memory.NewMemoryNode(memory.MemoryTypeHistory, "demo", "A photo of a sunset over the ocean.")
	textNode.Metadata["content_type"] = string(memory.ContentTypeText)

	imgNode := memory.NewMemoryNode(memory.MemoryTypeHistory, "demo", "sunset.jpg")
	imgNode.Metadata["content_type"] = string(memory.ContentTypeImage)
	imgNode.Metadata["url"] = "https://example.com/sunset.jpg"

	_ = store.Insert(ctx, []*memory.MemoryNode{textNode, imgNode})
	fmt.Println("seeded 2 multimodal memories")

	// 2. Create a cross-modal searcher.
	searcher := memory.NewCrossModalSearcher(store, embedder, &mockImageEmbedder{}, nil)

	// 3. Search with a text query; expect mixed results including images.
	query := "sunset ocean photo"
	results, err := searcher.SearchByText(ctx, query, 5)
	if err != nil {
		fmt.Println("search error:", err)
		return
	}
	fmt.Printf("query=%q results=%d\n", query, len(results))
	for _, r := range results {
		fmt.Printf("  type=%s score=%.3f content=%q url=%s\n", r.ContentType, r.Score, r.Content, r.URL)
	}
}
