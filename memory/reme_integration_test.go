package memory

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestReMeFileMemorySaveLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, err := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	if err != nil {
		t.Fatal(err)
	}
	m.SetLongTermMemory("prefs")
	if err := m.SaveTo("s1"); err != nil {
		t.Fatal(err)
	}
	m2, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	if err := m2.LoadFrom("s1"); err != nil {
		t.Fatal(err)
	}
	// long term restored
	m2.mu.RLock()
	lt := m2.longTerm
	m2.mu.RUnlock()
	if lt != "prefs" {
		t.Fatal(lt)
	}
}

func TestReMeVectorMemoryRetrieve(t *testing.T) {
	e := fixedEmbed{dim: 4}
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	n := NewMemoryNode(MemoryTypePersonal, "alice", "likes Go")
	if err := v.AddMemory(ctx, n); err != nil {
		t.Fatal(err)
	}
	out, err := v.RetrievePersonal(ctx, "alice", "Go", 5)
	if err != nil || len(out) != 1 {
		t.Fatalf("%v %v", out, err)
	}
}

func TestReMeVectorMemorySaveLoadSnapshot(t *testing.T) {
	e := fixedEmbed{dim: 4}
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	n := NewMemoryNode(MemoryTypePersonal, "bob", "vector persist")
	if err := v.AddMemory(ctx, n); err != nil {
		t.Fatal(err)
	}
	if err := v.SaveTo("sess"); err != nil {
		t.Fatal(err)
	}
	v2, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if err != nil {
		t.Fatal(err)
	}
	if err := v2.LoadFrom("sess"); err != nil {
		t.Fatal(err)
	}
	out, err := v2.RetrievePersonal(ctx, "bob", "persist", 5)
	if err != nil || len(out) != 1 {
		t.Fatalf("%v %v", out, err)
	}
}

type mockOrchestrator struct {
	summarizeResult *SummarizeResult
	retrieveResult  []*MemoryNode
}

func (m *mockOrchestrator) Summarize(ctx context.Context, msgs []*message.Msg, userName, taskName, toolName string) (*SummarizeResult, error) {
	return m.summarizeResult, nil
}

func (m *mockOrchestrator) Retrieve(ctx context.Context, query string, userName, taskName, toolName string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return m.retrieveResult, nil
}

func TestReMeVectorMemoryWithOrchestrator(t *testing.T) {
	e := fixedEmbed{dim: 4}
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockOrchestrator{
		summarizeResult: &SummarizeResult{
			PersonalMemories: []*MemoryNode{
				NewMemoryNode(MemoryTypePersonal, "alice", "喜欢简洁"),
			},
		},
		retrieveResult: []*MemoryNode{
			NewMemoryNode(MemoryTypePersonal, "alice", "喜欢简洁"),
		},
	}
	v.SetOrchestrator(mock)

	ctx := context.Background()
	msgs := []*message.Msg{}

	// SummarizeMemory
	sumRes, err := v.SummarizeMemory(ctx, msgs, "alice", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sumRes.PersonalMemories) != 1 {
		t.Fatalf("expected 1 personal memory, got %d", len(sumRes.PersonalMemories))
	}

	// RetrieveMemoryUnified
	retRes, err := v.RetrieveMemoryUnified(ctx, "简洁", "alice", "", "", RetrieveOptions{TopK: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(retRes) != 1 {
		t.Fatalf("expected 1 retrieve result, got %d", len(retRes))
	}

	// 测试未设置 orchestrator 时的错误
	v2, _ := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if _, err := v2.SummarizeMemory(ctx, msgs, "", "", ""); err == nil {
		t.Fatal("expected error when orchestrator not set")
	}
}
