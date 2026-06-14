// examples/embedding/main.go
//
// Demo: OpenAI-compatible embedding with file-based caching.
//
// This demo shows how to create an embedding model, wrap it with a disk cache,
// and perform batch embedding. It uses a mock/stub embedder so no API key is
// required to compile or run.
//
// How to run:
//   cd examples/embedding && go run main.go

package main

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/model"
)

// stubEmbedder is a mock embedder that returns deterministic vectors.
// Replace with embedding.NewOpenAI("sk-xxx", "") for real calls.
type stubEmbedder struct{}

func (s *stubEmbedder) ModelName() string { return "stub-embedder" }
func (s *stubEmbedder) Dimensions() int   { return 4 }
func (s *stubEmbedder) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	resp := &model.EmbeddingResponse{Object: "list", Model: s.ModelName()}
	for i, text := range input {
		vec := make([]float32, s.Dimensions())
		for j := range vec {
			vec[j] = float32(len(text) + j + i) // deterministic mock vector
		}
		resp.Data = append(resp.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: vec,
		})
	}
	return resp, nil
}

func main() {
	ctx := context.Background()

	// 1. Create a mock embedder (swap for embedding.NewOpenAI("sk-xxx", "")).
	base := &stubEmbedder{}

	// 2. Wrap with a file cache so repeated identical calls are instant.
	cached := embedding.WithFileCache(base, ".cache/embeddings")

	// 3. Batch embed a few strings.
	batch := []string{"hello world", "agentscope go", "embedding demo"}
	resp, err := cached.Embed(ctx, batch)
	if err != nil {
		fmt.Println("embed error:", err)
		return
	}

	fmt.Printf("model=%s batch=%d\n", resp.Model, len(resp.Data))
	for _, d := range resp.Data {
		fmt.Printf("  [%d] len=%d head=%v\n", d.Index, len(d.Embedding), d.Embedding[:min(3, len(d.Embedding))])
	}

	// 4. Single-text convenience helper.
	vec, err := embedding.Embed(ctx, cached, "single text")
	if err != nil {
		fmt.Println("single embed error:", err)
		return
	}
	fmt.Printf("single vector len=%d head=%v\n", len(vec), vec[:min(3, len(vec))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
