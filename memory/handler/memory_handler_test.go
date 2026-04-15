package handler

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/memory"
)

type fixedEmbed struct{ dim int }

func (f fixedEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, f.dim)
	for i := range v {
		v[i] = 0.1 * float32((i+1)%3)
	}
	_ = text
	return v, nil
}

func (f fixedEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for range texts {
		v, err := f.Embed(ctx, "")
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func TestMemoryHandlerCRUD(t *testing.T) {
	ctx := context.Background()
	store := memory.NewLocalVectorStore(fixedEmbed{dim: 4})
	h := NewMemoryHandler(store)

	node := memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "喜欢 Go")
	if err := h.AddMemory(ctx, node); err != nil {
		t.Fatal(err)
	}

	res, err := h.RetrieveMemory(ctx, "Go", memory.RetrieveOptions{TopK: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	node.Content = "喜欢 Go 语言"
	if err := h.UpdateMemory(ctx, node); err != nil {
		t.Fatal(err)
	}

	got, err := h.ListMemory(ctx, memory.MemoryTypePersonal, "alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 listed, got %d", len(got))
	}
	if got[0].Content != "喜欢 Go 语言" {
		t.Fatalf("unexpected content: %s", got[0].Content)
	}

	if err := h.DeleteMemory(ctx, node.MemoryID); err != nil {
		t.Fatal(err)
	}
	got2, _ := h.ListMemory(ctx, memory.MemoryTypePersonal, "alice", 10)
	if len(got2) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(got2))
	}
}

func TestMemoryHandlerDraftRetrieve(t *testing.T) {
	ctx := context.Background()
	store := memory.NewLocalVectorStore(fixedEmbed{dim: 4})
	h := NewMemoryHandler(store)

	existing := memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "喜欢喝咖啡")
	_ = h.AddMemory(ctx, existing)

	draft := memory.NewMemoryNode(memory.MemoryTypePersonal, "alice", "喜欢喝咖啡")
	similar, err := h.AddDraftAndRetrieveSimilar(ctx, draft, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(similar) != 1 {
		t.Fatalf("expected 1 similar, got %d", len(similar))
	}
}
