package embedding

import (
	"context"

	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/memory"
)

// LocalEmbedder wraps the top-level embedding.NewOllama for self-hosted models.
// This reduces duplication by delegating to the new embedding/ package.
type LocalEmbedder struct {
	inner embedding.Model
}

// NewLocalEmbedder creates an Ollama-backed local embedder (delegates to embedding.NewOllama).
func NewLocalEmbedder(endpoint, modelName string, dimension int) *LocalEmbedder {
	m := embedding.NewOllama(endpoint, modelName, dimension)
	return &LocalEmbedder{inner: m}
}

// Embed generates an embedding vector for a single text.
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.inner.Embed(ctx, []string{text})
	if err != nil || len(resp.Data) == 0 {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}

// EmbedBatch generates embedding vectors for multiple texts.
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	resp, err := e.inner.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

var _ memory.EmbeddingModel = (*LocalEmbedder)(nil)
