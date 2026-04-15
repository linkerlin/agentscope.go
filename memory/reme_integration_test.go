package memory

import (
	"context"
	"os"
	"path/filepath"
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
	defer m.Close()
	m.SetLongTermMemory("prefs")
	if err := m.SaveTo("s1"); err != nil {
		t.Fatal(err)
	}
	m2, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m2.Close()
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
	defer v.Close()
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
	defer v.Close()
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
	defer v2.Close()
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

func (m *mockOrchestrator) AddToolCallResult(result ToolCallResult) error {
	return nil
}

func (m *mockOrchestrator) SummarizeToolUsage(ctx context.Context, toolName string) error {
	return nil
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
	defer v.Close()

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

	// AddToolCallResult + SummarizeToolUsage
	if err := v.AddToolCallResult(ctx, ToolCallResult{ToolName: "grep", Input: map[string]any{}, Output: "ok", Success: true}); err != nil {
		t.Fatal(err)
	}
	if err := v.SummarizeToolUsage(ctx, "grep"); err != nil {
		t.Fatal(err)
	}

	// 测试未设置 orchestrator 时的错误
	v2, _ := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	defer v2.Close()
	if _, err := v2.SummarizeMemory(ctx, msgs, "", "", ""); err == nil {
		t.Fatal("expected error when orchestrator not set")
	}
	if err := v2.SummarizeToolUsage(ctx, "grep"); err == nil {
		t.Fatal("expected error when orchestrator not set")
	}
}

func TestReMeVectorMemoryFTSIntegration(t *testing.T) {
	e := fixedEmbed{dim: 4}
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	v, err := NewReMeVectorMemory(cfg, NewSimpleTokenCounter(), nil, e)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	ctx := context.Background()
	n1 := NewMemoryNode(MemoryTypePersonal, "alice", "用户喜欢 Go 语言")
	n2 := NewMemoryNode(MemoryTypePersonal, "alice", "用户热爱 Python 数据分析")
	if err := v.AddMemory(ctx, n1); err != nil {
		t.Fatal(err)
	}
	if err := v.AddMemory(ctx, n2); err != nil {
		t.Fatal(err)
	}

	// 验证 SQLite 文件已生成
	dbPath := filepath.Join(dir, ".agentscope", "reme.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected fts db file at %s", dbPath)
	}

	// 纯向量检索（VectorWeight=1）应返回 2 条
	resVector, err := v.RetrieveMemory(ctx, "Go", RetrieveOptions{TopK: 5, VectorWeight: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(resVector) != 2 {
		t.Fatalf("expected 2 vector results, got %d", len(resVector))
	}

	// 混合检索（VectorWeight=0.5）应返回按 BM25 融合排序的结果
	resHybrid, err := v.RetrieveMemory(ctx, "Go 语言", RetrieveOptions{TopK: 5, VectorWeight: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if len(resHybrid) != 2 {
		t.Fatalf("expected 2 hybrid results, got %d", len(resHybrid))
	}
	// 由于固定嵌入对两个文本返回相同向量，BM25 应决定排序："Go 语言" 更匹配查询
	if resHybrid[0].MemoryID != n1.MemoryID {
		t.Fatalf("expected n1 to rank first in hybrid, got %s", resHybrid[0].MemoryID)
	}

	// 测试删除后 FTS 同步
	if err := v.DeleteMemory(ctx, n1.MemoryID); err != nil {
		t.Fatal(err)
	}
	cnt, _ := v.fts.Count()
	if cnt != 1 {
		t.Fatalf("expected fts count 1 after delete, got %d", cnt)
	}
}
