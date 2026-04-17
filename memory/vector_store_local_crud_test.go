package memory

import (
	"context"
	"errors"
	"testing"
)

func TestLocalVectorStoreCRUD(t *testing.T) {
	e := fixedEmbed{dim: 2}
	s := NewLocalVectorStore(e)
	ctx := context.Background()

	_, err := s.Get(ctx, "missing")
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatal(err)
	}

	n := NewMemoryNode(MemoryTypePersonal, "u", "text")
	n.Vector = []float32{9, 9}
	if err := s.Insert(ctx, []*MemoryNode{n}); err != nil {
		t.Fatal(err)
	}
	g, err := s.Get(ctx, n.MemoryID)
	if err != nil || g.Content != "text" {
		t.Fatal(g, err)
	}

	n2 := *g
	n2.Content = "updated"
	if err := s.Update(ctx, &n2); err != nil {
		t.Fatal(err)
	}
	g2, _ := s.Get(ctx, n.MemoryID)
	if g2.Content != "updated" {
		t.Fatal(g2.Content)
	}

	if err := s.Delete(ctx, n.MemoryID); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get(ctx, n.MemoryID)
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatal(err)
	}

	_ = s.Insert(ctx, []*MemoryNode{NewMemoryNode(MemoryTypePersonal, "u", "a")})
	if err := s.DeleteAll(ctx); err != nil {
		t.Fatal(err)
	}
	res, _ := s.Search(ctx, "a", RetrieveOptions{TopK: 5, MinScore: 0})
	if len(res) != 0 {
		t.Fatal(len(res))
	}
}

func TestLocalVectorStoreInsertDimensionMismatch(t *testing.T) {
	e := fixedEmbed{dim: 2}
	s := NewLocalVectorStore(e)
	ctx := context.Background()
	n1 := NewMemoryNode(MemoryTypePersonal, "u", "first")
	if err := s.Insert(ctx, []*MemoryNode{n1}); err != nil {
		t.Fatal(err)
	}
	n2 := NewMemoryNode(MemoryTypePersonal, "u", "second")
	n2.Vector = []float32{1, 2, 3}
	err := s.Insert(ctx, []*MemoryNode{n2})
	if !errors.Is(err, ErrVectorDimension) {
		t.Fatalf("got %v", err)
	}
}

func TestLocalVectorStoreNilEmbed(t *testing.T) {
	s := NewLocalVectorStore(nil)
	err := s.Insert(context.Background(), []*MemoryNode{NewMemoryNode(MemoryTypePersonal, "u", "x")})
	if !errors.Is(err, ErrEmbeddingRequired) {
		t.Fatal(err)
	}
}


func TestLocalVectorStoreList(t *testing.T) {
	e := fixedEmbed{dim: 2}
	s := NewLocalVectorStore(e)
	ctx := context.Background()

	n1 := NewMemoryNode(MemoryTypePersonal, "alice", "a")
	n2 := NewMemoryNode(MemoryTypePersonal, "bob", "b")
	n3 := NewMemoryNode(MemoryTypeProcedural, "alice", "c")
	_ = s.Insert(ctx, []*MemoryNode{n1, n2, n3})

	list, err := s.List(MemoryTypePersonal, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 personal nodes, got %d", len(list))
	}

	filtered, err := s.List(MemoryTypePersonal, "alice", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 alice node, got %d", len(filtered))
	}

	limited, err := s.List(MemoryTypePersonal, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Fatalf("expected 1 limited node, got %d", len(limited))
	}
}
