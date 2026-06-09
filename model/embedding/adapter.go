package embedding

import (
	"context"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/model"
)

// FromMemory wraps a memory.EmbeddingModel as a model.EmbeddingModel.
func FromMemory(m memory.EmbeddingModel, name string, dimensions int) model.EmbeddingModel {
	if m == nil {
		return nil
	}
	if name == "" {
		name = "embedding"
	}
	return &memoryAdapter{m: m, name: name, dimensions: dimensions}
}

type memoryAdapter struct {
	m          memory.EmbeddingModel
	name       string
	dimensions int
}

func (a *memoryAdapter) ModelName() string { return a.name }
func (a *memoryAdapter) Dimensions() int   { return a.dimensions }

func (a *memoryAdapter) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	if len(input) == 0 {
		return &model.EmbeddingResponse{Object: "list", Model: a.name, Data: nil}, nil
	}
	vecs, err := a.m.EmbedBatch(ctx, input)
	if err != nil {
		return nil, err
	}
	resp := &model.EmbeddingResponse{
		Object: "list",
		Model:  a.name,
		Usage:  model.EmbeddingUsage{PromptTokens: len(input), TotalTokens: len(input)},
	}
	for i, vec := range vecs {
		dims := a.dimensions
		if dims <= 0 {
			dims = len(vec)
		}
		_ = dims
		resp.Data = append(resp.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: vec,
		})
	}
	return resp, nil
}

var _ model.EmbeddingModel = (*memoryAdapter)(nil)
