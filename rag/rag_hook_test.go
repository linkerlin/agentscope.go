package rag

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

type mockEmbedder struct {
	vec []float32
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.vec, nil
}

func TestSimpleMemoryRAG_StoreAndRetrieve(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	r := NewSimpleMemoryRAG(emb)
	ctx := context.Background()
	if err := r.Store(ctx, "d1", "hello world"); err != nil {
		t.Fatal(err)
	}
	msgs, err := r.Retrieve(ctx, "hello world", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].GetTextContent(), "hello world") {
		t.Fatalf("unexpected content: %s", msgs[0].GetTextContent())
	}
}

func TestRAGHook_OnEvent(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	rag := NewSimpleMemoryRAG(emb)
	ctx := context.Background()
	_ = rag.Store(ctx, "doc1", "Paris is the capital of France")

	h := NewRAGHook(rag, 1)
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("Where is Paris?").Build()
	hCtx := &hook.HookContext{
		Point:    hook.HookBeforeModel,
		Messages: []*message.Msg{userMsg},
	}

	res, err := h.OnEvent(ctx, hCtx)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.InjectMessages) != 2 {
		t.Fatalf("expected 2 messages, got %v", res)
	}
	if res.InjectMessages[0].Role != message.RoleSystem {
		t.Fatal("expected system message first")
	}
	text := res.InjectMessages[0].GetTextContent()
	if !strings.Contains(text, "Paris is the capital of France") {
		t.Fatalf("expected retrieved context in system message, got: %s", text)
	}
}

func TestRAGHook_AppendToExistingSystem(t *testing.T) {
	emb := &mockEmbedder{vec: []float32{1, 0, 0}}
	rag := NewSimpleMemoryRAG(emb)
	ctx := context.Background()
	_ = rag.Store(ctx, "doc1", "France info")

	h := NewRAGHook(rag, 1)
	sysMsg := message.NewMsg().Role(message.RoleSystem).TextContent("base").Build()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("France?").Build()
	hCtx := &hook.HookContext{
		Point:    hook.HookBeforeModel,
		Messages: []*message.Msg{sysMsg, userMsg},
	}

	res, err := h.OnEvent(ctx, hCtx)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.InjectMessages) != 2 {
		t.Fatalf("expected 2 messages, got %v", res)
	}
	text := res.InjectMessages[0].GetTextContent()
	if !strings.Contains(text, "base") || !strings.Contains(text, "France info") {
		t.Fatalf("expected merged system prompt, got: %s", text)
	}
}

func TestRAGHook_IgnoresOtherPoints(t *testing.T) {
	h := NewRAGHook(nil, 3)
	ctx := context.Background()
	hCtx := &hook.HookContext{Point: hook.HookAfterModel, Messages: []*message.Msg{}}
	res, err := h.OnEvent(ctx, hCtx)
	if err != nil || res != nil {
		t.Fatal("expected nil for non-before-model")
	}
}

func TestRAGHook_NoRetriever(t *testing.T) {
	h := NewRAGHook(nil, 3)
	ctx := context.Background()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	hCtx := &hook.HookContext{
		Point:    hook.HookBeforeModel,
		Messages: []*message.Msg{userMsg},
	}
	res, err := h.OnEvent(ctx, hCtx)
	if err != nil || res != nil {
		t.Fatal("expected nil when retriever is nil")
	}
}
