package embedding

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/memory"
)

// LocalEmbedder is a placeholder for local/self-hosted embedding models
// (e.g. Ollama, sentence-transformers, etc.).
// It satisfies the memory.EmbeddingModel interface but returns an error
// until a concrete local backend is wired in.
type LocalEmbedder struct {
	endpoint  string
	modelName string
	dimension int
}

// NewLocalEmbedder creates a placeholder local embedder.
func NewLocalEmbedder(endpoint, modelName string, dimension int) *LocalEmbedder {
	return &LocalEmbedder{
		endpoint:  endpoint,
		modelName: modelName,
		dimension: dimension,
	}
}

// Embed returns an error indicating that the local backend is not yet implemented.
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("embedding: local embedder not implemented yet (endpoint=%s model=%s)", e.endpoint, e.modelName)
}

// EmbedBatch returns an error indicating that the local backend is not yet implemented.
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("embedding: local embedder not implemented yet")
}

var _ memory.EmbeddingModel = (*LocalEmbedder)(nil)
