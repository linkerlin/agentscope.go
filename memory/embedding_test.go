package memory

import (
	"context"
	"testing"
)

type embedOnly struct{}

func (embedOnly) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{1, 0}, nil
}

func TestBatchFromEmbedder(t *testing.T) {
	m := BatchFromEmbedder(embedOnly{})
	ctx := context.Background()
	v, err := m.Embed(ctx, "a")
	if err != nil || len(v) != 2 {
		t.Fatal(v, err)
	}
	batch, err := m.EmbedBatch(ctx, []string{"a", "b"})
	if err != nil || len(batch) != 2 || len(batch[0]) != 2 {
		t.Fatal(batch, err)
	}
}
