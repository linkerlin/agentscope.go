package pipeline

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/memory"
)

type mockStep struct {
	name     string
	executed *bool
}

func (m *mockStep) Name() string                                    { return m.name }
func (m *mockStep) Execute(_ context.Context, _ *FlowContext) error { *m.executed = true; return nil }

func TestSeqExecutionOrder(t *testing.T) {
	var s1, s2, s3 bool
	p := NewPipeline("test-seq", Seq(
		&mockStep{"s1", &s1},
		&mockStep{"s2", &s2},
		&mockStep{"s3", &s3},
	))
	err := p.Execute(context.Background(), NewFlowContext("test"))
	if err != nil {
		t.Fatal(err)
	}
	if !s1 || !s2 || !s3 {
		t.Fatal("all steps should be executed")
	}
}

func TestParExecution(t *testing.T) {
	var s1, s2, s3 bool
	p := NewPipeline("test-par", Par(
		&mockStep{"s1", &s1},
		&mockStep{"s2", &s2},
		&mockStep{"s3", &s3},
	))
	err := p.Execute(context.Background(), NewFlowContext("test"))
	if err != nil {
		t.Fatal(err)
	}
	if !s1 || !s2 || !s3 {
		t.Fatal("all steps should be executed")
	}
}

func TestPipelineWithMemoryRetrieval(t *testing.T) {
	embed := &mockEmbedder{}
	store := memory.NewLocalVectorStore(embed)

	node := &memory.MemoryNode{
		MemoryID:   "test1",
		Content:    "Python is a programming language",
		MemoryType: memory.MemoryTypePersonal,
		Vector:     []float32{1, 0, 0},
	}
	_ = store.Insert(context.Background(), []*memory.MemoryNode{node})

	p := NewPipeline("retrieve-task-memory", Seq(
		&MemoryRetrievalStep{Store: store},
		&MemoryValidationStep{Threshold: 0.0},
	))

	fc := NewFlowContext("Python")
	fc.TopK = 5
	err := p.Execute(context.Background(), fc)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.RetrievedNodes) == 0 {
		t.Fatal("should retrieve at least one node")
	}
}

func TestPipelineWithDedup(t *testing.T) {
	dedup := memory.NewMemoryDeduplicator(nil)
	dedup.SimilarityThreshold = 0.5

	nodes := []*memory.MemoryNode{
		{MemoryID: "a", Content: "Python", Vector: []float32{1, 0, 0}},
		{MemoryID: "b", Content: "Python", Vector: []float32{1, 0, 0}},
		{MemoryID: "c", Content: "Go", Vector: []float32{0, 1, 0}},
	}

	p := NewPipeline("test-dedup", Seq(
		&MemoryDeduplicationStep{Dedup: dedup},
	))

	fc := NewFlowContext("")
	fc.MemoryNodes = nodes
	err := p.Execute(context.Background(), fc)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.DedupedNodes) < 2 || len(fc.DedupedNodes) > 3 {
		t.Fatalf("expected 2-3 deduped, got %d", len(fc.DedupedNodes))
	}
}

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if text == "Python" {
		return []float32{1, 0, 0}, nil
	}
	return []float32{0, 0, 1}, nil
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		v, _ := m.Embed(context.Background(), t)
		result[i] = v
	}
	return result, nil
}
