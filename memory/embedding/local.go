package embedding

import (
	"context"

	modelembed "github.com/linkerlin/agentscope.go/model/embedding"
	"github.com/linkerlin/agentscope.go/memory"
)

// LocalEmbedder delegates to OllamaEmbedder for self-hosted embedding models.
type LocalEmbedder struct {
	inner *modelembed.OllamaEmbedder
}

// NewLocalEmbedder creates an Ollama-backed local embedder.
func NewLocalEmbedder(endpoint, modelName string, dimension int) *LocalEmbedder {
	return &LocalEmbedder{inner: modelembed.NewOllamaEmbedder(endpoint, modelName, dimension)}
}

// Embed generates an embedding vector for a single text.
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.inner.EmbedSingle(ctx, text)
}

// EmbedBatch generates embedding vectors for multiple texts.
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return e.inner.EmbedBatch(ctx, texts)
}

var _ memory.EmbeddingModel = (*LocalEmbedder)(nil)
