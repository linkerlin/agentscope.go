package memory

import (
	"context"
	"testing"
)

type fixedEmbed struct {
	dim int
}

func (f fixedEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, f.dim)
	for i := range v {
		v[i] = 0.1 * float32(i+1)
	}
	_ = text
	return v, nil
}

func (f fixedEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for range texts {
		v, _ := f.Embed(ctx, "")
		out = append(out, v)
	}
	return out, nil
}

func TestLocalVectorStoreInsertSearch(t *testing.T) {
	e := fixedEmbed{dim: 4}
	s := NewLocalVectorStore(e)
	ctx := context.Background()
	n := NewMemoryNode(MemoryTypePersonal, "u1", "hello world")
	if err := s.Insert(ctx, []*MemoryNode{n}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Search(ctx, "hello", RetrieveOptions{TopK: 5, MinScore: 0})
	if err != nil || len(res) != 1 {
		t.Fatalf("%v %v", res, err)
	}
}
