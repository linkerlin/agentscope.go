package embedding

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
)

func TestNewOpenAI(t *testing.T) {
	// Just construct; real call needs key. We test interface compliance and name.
	m := NewOpenAI("sk-fake", "")
	if m.ModelName() == "" {
		t.Error("expected model name")
	}
	if m.Dimensions() <= 0 {
		t.Error("expected positive dimensions")
	}
	var _ model.EmbeddingModel = m
}

func TestNewOllama(t *testing.T) {
	m := NewOllama("", "nomic-embed-text", 768)
	if m.ModelName() != "nomic-embed-text" {
		t.Error("name mismatch")
	}
	if m.Dimensions() != 768 {
		t.Error("dim mismatch")
	}
	var _ model.EmbeddingModel = m
}

func TestWithFileCache(t *testing.T) {
	// Use a mock that always returns fixed vec.
	mock := &fixedModel{name: "test-model", dim: 4, vec: []float32{0.1, 0.2, 0.3, 0.4}}
	cached := WithFileCache(mock, t.TempDir())

	// First call should hit inner
	resp1, err := cached.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp1.Data) != 1 || len(resp1.Data[0].Embedding) != 4 {
		t.Fatalf("bad first response: %+v", resp1)
	}

	// Second call should come from cache (we can't easily assert without spy,
	// but at least it shouldn't error and returns same shape).
	resp2, err := cached.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Model != "test-model" {
		t.Error("model name lost in cache path")
	}
}

type fixedModel struct {
	name string
	dim  int
	vec  []float32
}

func (f *fixedModel) ModelName() string { return f.name }
func (f *fixedModel) Dimensions() int   { return f.dim }
func (f *fixedModel) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	resp := &model.EmbeddingResponse{
		Object: "list",
		Model:  f.name,
	}
	for i := range input {
		resp.Data = append(resp.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: append([]float32(nil), f.vec...),
		})
	}
	return resp, nil
}

var _ model.EmbeddingModel = (*fixedModel)(nil)

func TestNewGemini(t *testing.T) {
	m := NewGemini("fake-key", "")
	if m.ModelName() == "" {
		t.Error("gemini name")
	}
	var _ model.EmbeddingModel = m
}

func TestNewDashScope(t *testing.T) {
	m := NewDashScope("fake-key", "")
	if m.ModelName() == "" {
		t.Error("dashscope name")
	}
	var _ model.EmbeddingModel = m
}
