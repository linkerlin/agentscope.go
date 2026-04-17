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


func TestMemoryDeduplicatorWithLLM(t *testing.T) {
	ctx := context.Background()
	embed := &mockEmbeddingModel{}
	llm := &mockSummarizerModel{response: "<1> <重复>\n<2> <保留>"}
	d := NewMemoryDeduplicator(embed).WithLLM(llm)
	d.SimilarityThreshold = 0.5

	m1 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢咖啡")
	m1.Vector = []float32{1, 0, 0, 0}
	m2 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢拿铁")
	m2.Vector = []float32{0, 1, 0, 0}
	m3 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢茶")
	m3.Vector = []float32{0, 0, 1, 0}

	unique, removed, err := d.Deduplicate(ctx, []*MemoryNode{m1, m2, m3})
	if err != nil {
		t.Fatal(err)
	}
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique after LLM dedup, got %d", len(unique))
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}
}

func TestMemoryDeduplicatorFindContradictions(t *testing.T) {
	ctx := context.Background()
	embed := &mockEmbeddingModel{}
	llm := &mockSummarizerModel{response: "矛盾: <1>, <2>\n原因: 相反"}
	d := NewMemoryDeduplicator(embed).WithLLM(llm)

	m1 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢咖啡")
	m2 := NewMemoryNode(MemoryTypePersonal, "alice", "讨厌咖啡")

	pairs, err := d.FindContradictions(ctx, []*MemoryNode{m1, m2})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 contradiction pair, got %d", len(pairs))
	}
}

func TestMemoryDeduplicatorMergeSimilarMemories(t *testing.T) {
	ctx := context.Background()
	embed := &mockEmbeddingModel{}
	llm := &mockSummarizerModel{response: "喜欢咖啡和茶"}
	d := NewMemoryDeduplicator(embed).WithLLM(llm)
	d.SimilarityThreshold = 0.99

	m1 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢咖啡")
	m1.Vector = []float32{1, 0, 0, 0}
	m2 := NewMemoryNode(MemoryTypePersonal, "alice", "喜欢茶")
	m2.Vector = []float32{0, 1, 0, 0}

	merged, err := d.MergeSimilarMemories(ctx, []*MemoryNode{m1, m2})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 2 {
		t.Fatalf("expected 2 memories (no merge), got %d", len(merged))
	}

	d.SimilarityThreshold = 0.1
	// Use identical vectors to force grouping
	m1.Vector = []float32{1, 0, 0, 0}
	m2.Vector = []float32{1, 0, 0, 0}
	merged2, err := d.MergeSimilarMemories(ctx, []*MemoryNode{m1, m2})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged2) != 1 {
		t.Fatalf("expected 1 merged memory, got %d", len(merged2))
	}
}

func TestMemoryDeduplicatorParseResponses(t *testing.T) {
	d := NewMemoryDeduplicator(&mockEmbeddingModel{})
	memories := []*MemoryNode{
		{MemoryID: "1", Content: "a"},
		{MemoryID: "2", Content: "b"},
	}
	filtered, removed, err := d.parseDeduplicationResponse("<1> <contained>", memories)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || len(removed) != 1 || removed[0] != "1" {
		t.Fatalf("unexpected parse result: %v %v", filtered, removed)
	}

	contradictions, err := d.parseContradictionResponse("Contradiction: <1>, <2>", memories)
	if err != nil {
		t.Fatal(err)
	}
	if len(contradictions) != 1 {
		t.Fatalf("expected 1 contradiction, got %d", len(contradictions))
	}
}

func TestMemoryDeduplicatorCalculateTextSimilarity(t *testing.T) {
	d := NewMemoryDeduplicator(&mockEmbeddingModel{})
	sim := d.calculateTextSimilarity("hello world", "hello world")
	if sim != 1.0 {
		t.Fatalf("expected 1.0 for identical text, got %f", sim)
	}
	sim2 := d.calculateTextSimilarity("abc", "xyz")
	if sim2 != 0.0 {
		t.Fatalf("expected 0.0 for unrelated text, got %f", sim2)
	}
}
