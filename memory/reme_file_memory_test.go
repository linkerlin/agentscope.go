package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestReMeFileMemoryCheckContextAndRecent(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, err := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for i := 0; i < 5; i++ {
		_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("line").Build())
	}
	cc, err := m.CheckContext(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if cc.TotalTokens <= 1 {
		t.Fatal("expected over threshold")
	}
	recent, err := m.GetRecent(2)
	if err != nil || len(recent) != 2 {
		t.Fatal(len(recent), err)
	}
	if err := m.Clear(); err != nil {
		t.Fatal(err)
	}
	if m.Size() != 0 {
		t.Fatal(m.Size())
	}
}

func TestReMeFileMemoryCompactMemoryNoModel(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	_, err := m.CompactMemory(context.Background(), []*message.Msg{}, CompactOptions{})
	if !errors.Is(err, ErrCompactorNoModel) {
		t.Fatal(err)
	}
}

func TestReMeFileMemoryInitCompactor(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	m.InitCompactorWithModel(&mockChatModel{})
	_, err := m.CompactMemory(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("x").Build(),
	}, CompactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m.InitCompactorWithModel(nil)
	_, err = m.CompactMemory(context.Background(), nil, CompactOptions{})
	if !errors.Is(err, ErrCompactorNoModel) {
		t.Fatal(err)
	}
}

func TestReMeFileMemorySaveToEmptyID(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	if err := m.SaveTo(""); err == nil {
		t.Fatal("expected error")
	}
}


func TestReMeFileMemoryEstimateTokens(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	stats, err := m.EstimateTokens([]*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello world").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalMessages != 1 || stats.EstimatedTokens <= 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestReMeFileMemoryInitSummarizerWithModel(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	m.InitSummarizerWithModel(&mockChatModel{reply: "summary"})
	if m.summarizer == nil {
		t.Fatal("expected summarizer")
	}
	m.InitSummarizerWithModel(nil)
	if m.summarizer != nil {
		t.Fatal("expected nil summarizer")
	}
}

func TestReMeFileMemoryGetMemoryForPrompt(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("msg1").Build())
	m.SetLongTermMemory("ltm")
	m.SetCompSum("compsum")

	msgs, err := m.GetMemoryForPrompt(true)
	if err != nil || len(msgs) != 2 {
		t.Fatal(err, len(msgs))
	}
	if !strings.Contains(msgs[0].GetTextContent(), "ltm") {
		t.Fatal("expected ltm in prepend msg")
	}

	msgs2, err := m.GetMemoryForPrompt(false)
	if err != nil || len(msgs2) != 1 {
		t.Fatal(err, len(msgs2))
	}
}

func TestReMeFileMemoryGetMemoryForPrompt_Empty(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("msg1").Build())
	msgs, err := m.GetMemoryForPrompt(true)
	if err != nil || len(msgs) != 1 {
		t.Fatal(err, len(msgs))
	}
}

func TestReMeFileMemoryPreReasoningPrepare(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	cfg.MaxInputLength = 100
	cfg.MemoryCompactReserve = 10
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	m.InitCompactorWithModel(&mockChatModel{reply: sampleCompactMarkdown()})

	// empty history
	out, sum, err := m.PreReasoningPrepare(context.Background(), nil)
	if err != nil || len(out) != 0 || sum != nil {
		t.Fatal("expected nil for empty history")
	}

	// no compaction needed
	h := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}
	out, sum, err = m.PreReasoningPrepare(context.Background(), h)
	if err != nil || sum != nil {
		t.Fatal("expected nil summary when under threshold")
	}

	// exceeds threshold
	for i := 0; i < 20; i++ {
		_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("this is a long message to exceed token threshold quickly").Build())
	}
	h = m.Msgs()
	out, sum, err = m.PreReasoningPrepare(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if sum == nil || sum.Raw == "" {
		t.Fatal("expected compact summary")
	}
	if len(out) == 0 {
		t.Fatal("expected output messages")
	}
}

func TestReMeFileMemoryPreReasoningPrepare_NoCompactor(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	cfg.MaxInputLength = 100
	cfg.MemoryCompactReserve = 10
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	for i := 0; i < 20; i++ {
		_ = m.Add(message.NewMsg().Role(message.RoleUser).TextContent("this is a long message to exceed token threshold quickly").Build())
	}
	h := m.Msgs()
	out, sum, err := m.PreReasoningPrepare(context.Background(), h)
	if err != nil || sum != nil {
		t.Fatal("expected nil summary when no compactor")
	}
	if len(out) == 0 {
		t.Fatal("expected output messages")
	}
}

func TestReMeFileMemoryAsyncSummaryTask(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	m.InitSummarizerWithModel(&mockChatModel{reply: "summary"})
	m.AddAsyncSummaryTask(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	})
	if err := m.AwaitSummaryTasks(); err != nil {
		t.Fatal(err)
	}
}

func TestReMeFileMemoryAsyncSummaryTask_NoModel(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	m.AddAsyncSummaryTask(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	})
	if err := m.AwaitSummaryTasks(); err != nil {
		t.Fatal(err)
	}
}

func TestReMeFileMemoryFTSIndex(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultReMeFileConfig()
	cfg.WorkingDir = dir
	m, _ := NewReMeFileMemory(cfg, NewSimpleTokenCounter())
	defer m.Close()
	fts := m.FTSIndex()
	if fts == nil {
		t.Fatal("expected fts index")
	}
}
