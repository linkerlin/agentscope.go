package memory

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLocalVectorStoreSnapshotRoundTrip(t *testing.T) {
	e := fixedEmbed{dim: 3}
	s := NewLocalVectorStore(e)
	ctx := context.Background()
	n := NewMemoryNode(MemoryTypePersonal, "u", "hello")
	if err := s.Insert(ctx, []*MemoryNode{n}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := s.WriteSnapshot(path); err != nil {
		t.Fatal(err)
	}
	s2 := NewLocalVectorStore(e)
	if err := s2.ReadSnapshot(path); err != nil {
		t.Fatal(err)
	}
	res, err := s2.Search(ctx, "hello", RetrieveOptions{TopK: 5, MinScore: 0})
	if err != nil || len(res) != 1 {
		t.Fatalf("%v %v", res, err)
	}
}
