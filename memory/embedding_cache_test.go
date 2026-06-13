package memory

import (
	"context"
	"testing"
)

type countingEmbedder struct {
	callCount int
	v         []float32
}

func (c *countingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	c.callCount++
	return c.v, nil
}

func (c *countingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	c.callCount += len(texts)
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = c.v
	}
	return out, nil
}

func TestEmbeddingCacheHitMiss(t *testing.T) {
	base := &countingEmbedder{v: []float32{0.1, 0.2, 0.3}}
	cache := NewEmbeddingCache(base, 2)
	ctx := context.Background()

	v1, _ := cache.Embed(ctx, "hello")
	if len(v1) != 3 {
		t.Fatal("unexpected vector len")
	}
	if base.callCount != 1 {
		t.Fatalf("expected 1 embed call, got %d", base.callCount)
	}

	// cache hit
	_, _ = cache.Embed(ctx, "hello")
	if base.callCount != 1 {
		t.Fatalf("expected 1 embed call after hit, got %d", base.callCount)
	}

	hits, misses := cache.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("unexpected stats: hits=%d misses=%d", hits, misses)
	}
}

func TestEmbeddingCacheLRUEviction(t *testing.T) {
	base := &countingEmbedder{v: []float32{0.1}}
	cache := NewEmbeddingCache(base, 2)
	ctx := context.Background()

	_, _ = cache.Embed(ctx, "a")
	_, _ = cache.Embed(ctx, "b")
	_, _ = cache.Embed(ctx, "c") // evicts "a"

	if base.callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", base.callCount)
	}

	// "a" should be evicted
	_, _ = cache.Embed(ctx, "a")
	if base.callCount != 4 {
		t.Fatalf("expected 4 calls after re-embedding a, got %d", base.callCount)
	}
}

func TestEmbeddingCacheEmbedBatch(t *testing.T) {
	base := &countingEmbedder{v: []float32{0.1}}
	cache := NewEmbeddingCache(base, 10)
	ctx := context.Background()

	_, _ = cache.EmbedBatch(ctx, []string{"x", "y"})
	_, _ = cache.EmbedBatch(ctx, []string{"x", "y", "z"})

	if base.callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", base.callCount)
	}
}
