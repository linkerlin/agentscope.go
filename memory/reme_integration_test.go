package memory

import (
	"context"
	"testing"
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
