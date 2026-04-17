package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestMessagesToMemoryNodes(t *testing.T) {
	u := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	msgs := []*message.Msg{nil, u}
	out := MessagesToMemoryNodes(msgs, MemoryTypePersonal, "alice")
	if len(out) != 1 || out[0].MemoryTarget != "alice" {
		t.Fatal(len(out))
	}
}


func TestReMeVectorMemory_UpdateMemory(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, &mockEmbeddingModel{})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	node := NewMemoryNode(MemoryTypePersonal, "u", "content")
	if err := v.AddMemory(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	node.Content = "updated"
	if err := v.UpdateMemory(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	got, err := v.store.Get(context.Background(), node.MemoryID)
	if err != nil || got.Content != "updated" {
		t.Fatal(err, got)
	}
}

func TestReMeVectorMemory_VectorStore(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, &mockEmbeddingModel{})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if v.VectorStore() == nil {
		t.Fatal("expected vector store")
	}
}

func TestNewReMeVectorMemoryWithOrchestrator(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemoryWithOrchestrator(cfg, NewSimpleTokenCounter(), nil, &mockEmbeddingModel{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if v == nil {
		t.Fatal("expected memory")
	}
}
