package memory

import (
	"context"
	"errors"
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
