package handler

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

func TestHistoryHandler(t *testing.T) {
	ctx := context.Background()
	store := memory.NewLocalVectorStore(fixedEmbed{dim: 4})
	h := NewHistoryHandler(store)

	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("你好").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("你好！").Build(),
	}

	node, err := h.AddHistory(ctx, msgs, "alice", "qwen")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected node, got nil")
	}
	if node.MemoryTarget != "alice" {
		t.Fatalf("unexpected target: %s", node.MemoryTarget)
	}

	hist, err := h.ReadHistory(ctx, "alice", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 history node, got %d", len(hist))
	}
}
