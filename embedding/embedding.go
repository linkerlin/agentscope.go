package embedding

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
)

// Model is an alias for the core embedding interface for convenience.
// Users can import "github.com/linkerlin/agentscope.go/embedding" and get
// clean constructors + cache without digging into model/ or memory/.
type Model = model.EmbeddingModel

// Embed is a helper for single-text convenience (wraps batch).
func Embed(ctx context.Context, m Model, text string) ([]float32, error) {
	resp, err := m.Embed(ctx, []string{text})
	if err != nil || len(resp.Data) == 0 {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}

// WithFileCache wraps any Model with a simple file-based cache.
// Identifiers are hashed (model + input) so repeated identical calls are fast and cheap.
// This mirrors Python's FileEmbeddingCache for cost and latency reduction.
// Supports all providers including new Gemini and DashScope (multimodal models noted for future).
func WithFileCache(m Model, cacheDir string) Model {
	if cacheDir == "" {
		cacheDir = ".cache/embeddings"
	}
	c := &fileCache{
		inner:    m,
		cacheDir: cacheDir,
	}
	return c
}

// fileCache implements Model with on-disk caching.
type fileCache struct {
	inner    Model
	cacheDir string
}

func (c *fileCache) ModelName() string { return c.inner.ModelName() }
func (c *fileCache) Dimensions() int   { return c.inner.Dimensions() }

func (c *fileCache) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	// For simplicity, we cache per full batch request key.
	// In real use you may want per-text caching, but batch-level is common and simple.
	key := cacheKey(c.inner.ModelName(), input)

	if cached, ok := loadCache(c.cacheDir, key); ok && len(cached) == len(input) {
		resp := &model.EmbeddingResponse{
			Object: "list",
			Model:  c.inner.ModelName(),
			Data:   cached,
		}
		return resp, nil
	}

	resp, err := c.inner.Embed(ctx, input)
	if err != nil {
		return nil, err
	}
	_ = saveCache(c.cacheDir, key, resp.Data)
	return resp, nil
}

var _ Model = (*fileCache)(nil)
