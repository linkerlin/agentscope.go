package kb

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// stubEmbedder maps text to a deterministic 3-dim vector for testing.
type stubEmbedder struct{ dim int }

func (e *stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.dim <= 0 {
		e.dim = 3
	}
	v := make([]float32, e.dim)
	for i := 0; i < e.dim; i++ {
		v[i] = float32(len(text) * (i + 1))
	}
	return v, nil
}

func TestKnowledgeBase_InsertSearchDelete(t *testing.T) {
	store := NewInMemoryVectorStore()
	mgr := NewCollectionPerKBManager(store, func(id string) (Embedder, error) {
		return &stubEmbedder{dim: 3}, nil
	})
	ctx := context.Background()

	if err := mgr.Create(ctx, Spec{Name: "docs", Description: "test kb", EmbedderID: "stub"}); err != nil {
		t.Fatal(err)
	}
	kb, err := mgr.Get(ctx, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if kb.Collection != "kb_docs" {
		t.Fatalf("collection = %q", kb.Collection)
	}

	chunks := []document.Chunk{
		{Content: message.NewTextBlock("alpha content"), Source: "a.txt", ChunkIndex: 0, TotalChunks: 2},
		{Content: message.NewTextBlock("beta content"), Source: "a.txt", ChunkIndex: 1, TotalChunks: 2},
	}
	if err := kb.InsertChunks(ctx, "doc1", chunks); err != nil {
		t.Fatal(err)
	}

	docs, err := kb.ListDocuments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].DocID != "doc1" || docs[0].Chunks != 2 {
		t.Fatalf("unexpected docs: %+v", docs)
	}

	results, err := kb.Search(ctx, "alpha content", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].Text != "alpha content" {
		t.Fatalf("top result = %q", results[0].Text)
	}

	if err := kb.DeleteDocument(ctx, "doc1"); err != nil {
		t.Fatal(err)
	}
	docs, _ = kb.ListDocuments(ctx)
	if len(docs) != 0 {
		t.Fatalf("expected no docs after delete, got %+v", docs)
	}
}

func TestKnowledgeBase_MultiTenantFilter(t *testing.T) {
	store := NewInMemoryVectorStore()
	mgr := NewCollectionPerKBManager(store, func(id string) (Embedder, error) {
		return &stubEmbedder{dim: 2}, nil
	})
	ctx := context.Background()

	// Two KBs in the SAME collection but different tenant filters.
	mgr.Create(ctx, Spec{Name: "t1", Collection: "shared", EmbedderID: "s", Filter: map[string]any{"tenant": "a"}})
	mgr.Create(ctx, Spec{Name: "t2", Collection: "shared", EmbedderID: "s", Filter: map[string]any{"tenant": "b"}})

	kbA, _ := mgr.Get(ctx, "t1")
	kbB, _ := mgr.Get(ctx, "t2")

	kbA.InsertChunks(ctx, "da", []document.Chunk{{Content: message.NewTextBlock("secret A"), Source: "a"}})
	kbB.InsertChunks(ctx, "db", []document.Chunk{{Content: message.NewTextBlock("secret B"), Source: "b"}})

	// Tenant A cannot see tenant B's documents.
	docsA, _ := kbA.ListDocuments(ctx)
	docsB, _ := kbB.ListDocuments(ctx)
	if len(docsA) != 1 || docsA[0].DocID != "da" {
		t.Fatalf("tenant A leaked: %+v", docsA)
	}
	if len(docsB) != 1 || docsB[0].DocID != "db" {
		t.Fatalf("tenant B leaked: %+v", docsB)
	}
	// search is also scoped
	resA, _ := kbA.Search(ctx, "secret", 10)
	for _, r := range resA {
		if r.Metadata["tenant"] != "a" {
			t.Fatal("tenant filter not enforced on search")
		}
	}
}

func TestKBManager_CreateDuplicate(t *testing.T) {
	mgr := NewCollectionPerKBManager(NewInMemoryVectorStore(), func(string) (Embedder, error) {
		return &stubEmbedder{}, nil
	})
	ctx := context.Background()
	mgr.Create(ctx, Spec{Name: "x", EmbedderID: "s"})
	if err := mgr.Create(ctx, Spec{Name: "x", EmbedderID: "s"}); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestKBManager_GetMissing(t *testing.T) {
	mgr := NewCollectionPerKBManager(NewInMemoryVectorStore(), func(string) (Embedder, error) {
		return &stubEmbedder{}, nil
	})
	if _, err := mgr.Get(context.Background(), "nope"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestKBManager_DeleteDropsCollection(t *testing.T) {
	store := NewInMemoryVectorStore()
	mgr := NewCollectionPerKBManager(store, func(string) (Embedder, error) { return &stubEmbedder{}, nil })
	ctx := context.Background()
	mgr.Create(ctx, Spec{Name: "kb1", EmbedderID: "s"})
	kb, _ := mgr.Get(ctx, "kb1")
	kb.InsertChunks(ctx, "d", []document.Chunk{{Content: message.NewTextBlock("hi"), Source: "f"}})

	if err := mgr.Delete(ctx, "kb1"); err != nil {
		t.Fatal(err)
	}
	store.mu.RLock()
	_, exists := store.collections["kb_kb1"]
	store.mu.RUnlock()
	if exists {
		t.Fatal("collection not dropped")
	}
}

func TestEmbedderFactoryError(t *testing.T) {
	mgr := NewCollectionPerKBManager(NewInMemoryVectorStore(), func(id string) (Embedder, error) {
		return nil, errors.New("no such embedder")
	})
	ctx := context.Background()
	mgr.Create(ctx, Spec{Name: "x", EmbedderID: "bad"})
	if _, err := mgr.Get(ctx, "x"); err == nil {
		t.Fatal("expected embedder resolution error")
	}
}

func TestInsertChunks_SkipsEmpty(t *testing.T) {
	store := NewInMemoryVectorStore()
	mgr := NewCollectionPerKBManager(store, func(string) (Embedder, error) { return &stubEmbedder{}, nil })
	ctx := context.Background()
	mgr.Create(ctx, Spec{Name: "k", EmbedderID: "s"})
	kb, _ := mgr.Get(ctx, "k")
	// DataBlock-only chunk (no text) should be skipped, not error.
	if err := kb.InsertChunks(ctx, "d", []document.Chunk{{
		Content: message.NewDataBlock(message.TypeImage, &message.Source{URL: "x"}),
	}}); err != nil {
		t.Fatal(err)
	}
	docs, _ := kb.ListDocuments(ctx)
	if len(docs) != 0 {
		t.Fatalf("expected no text records, got %+v", docs)
	}
}

// compile-time assertions
var _ VectorStore = (*InMemoryVectorStore)(nil)
var _ Embedder = (*stubEmbedder)(nil)
