package memory

import (
	"context"
	"testing"
)

func TestDeduplicateAgainstStore(t *testing.T) {
	ctx := context.Background()
	embed := &mockEmbeddingModel{}
	store := NewLocalVectorStore(embed)

	existing := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢喝咖啡")
	if err := store.Insert(ctx, []*MemoryNode{existing}); err != nil {
		t.Fatal(err)
	}

	d := NewMemoryDeduplicator(embed)
	d.SimilarityThreshold = 0.5

	newMem := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢喝咖啡")
	unique, err := d.DeduplicateAgainstStore(ctx, []*MemoryNode{newMem}, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(unique) != 0 {
		t.Fatalf("expected 0 unique after dedup against store, got %d", len(unique))
	}
}

func TestMemoryDeduplicateByVector(t *testing.T) {
	ctx := context.Background()
	embed := &mockEmbeddingModel{}
	d := NewMemoryDeduplicator(embed)
	d.SimilarityThreshold = 0.99 // 极高阈值，只有完全相同向量的才被视为重复

	m1 := NewMemoryNode(MemoryTypePersonal, "alice", "test")
	m2 := NewMemoryNode(MemoryTypePersonal, "alice", "test")
	m3 := NewMemoryNode(MemoryTypePersonal, "alice", "other")

	unique, removed, err := d.Deduplicate(ctx, []*MemoryNode{m1, m2, m3})
	if err != nil {
		t.Fatal(err)
	}
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique, got %d", len(unique))
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}
}
