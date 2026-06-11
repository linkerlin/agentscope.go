package embedding

import (
	"context"
	"fmt"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/embedding"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/model"
	modelembed "github.com/linkerlin/agentscope.go/model/embedding"
)

// OpenAIEmbedder uses the top-level embedding package under the hood
// to generate text embeddings (reduces duplication with new embedding/ package).
// Fields kept for test compatibility.
type OpenAIEmbedder struct {
	inner     embedding.Model
	client    *goopenai.Client // for test compat / AsModel
	modelName string
}

// NewOpenAIEmbedder creates an embedder with the given API key and model.
// Delegates to embedding.NewOpenAI for the core implementation.
func NewOpenAIEmbedder(apiKey, modelName string) *OpenAIEmbedder {
	if modelName == "" {
		modelName = "text-embedding-3-small"
	}
	innerM := embedding.NewOpenAI(apiKey, modelName)
	// keep client for compat with existing tests / AsModel
	client := goopenai.NewClient(apiKey)
	return &OpenAIEmbedder{
		inner:     innerM,
		client:    client,
		modelName: modelName,
	}
}

// NewOpenAIEmbedderWithBaseURL creates an embedder with a custom base URL.
// Delegates to embedding.NewOpenAIWithBaseURL.
func NewOpenAIEmbedderWithBaseURL(apiKey, baseURL, modelName string) *OpenAIEmbedder {
	if modelName == "" {
		modelName = "text-embedding-3-small"
	}
	innerM := embedding.NewOpenAIWithBaseURL(apiKey, baseURL, modelName)
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := goopenai.NewClientWithConfig(cfg)
	return &OpenAIEmbedder{
		inner:     innerM,
		client:    client,
		modelName: modelName,
	}
}

// Embed generates an embedding vector for a single text (delegates to inner if available, fallback for test compat).
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.inner != nil {
		resp, err := e.inner.Embed(ctx, []string{text})
		if err != nil {
			return nil, fmt.Errorf("openai embedding: %w", err)
		}
		if len(resp.Data) == 0 {
			return nil, fmt.Errorf("openai embedding: no data returned")
		}
		return resp.Data[0].Embedding, nil
	}
	// fallback for direct test construction with client
	if e.client == nil {
		return nil, fmt.Errorf("openai embedding: no client")
	}
	resp, err := e.client.CreateEmbeddings(ctx, goopenai.EmbeddingRequestStrings{
		Input: []string{text},
		Model: goopenai.EmbeddingModel(e.modelName),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embedding: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embedding: no data returned")
	}
	return resp.Data[0].Embedding, nil
}

// EmbedBatch generates embedding vectors for multiple texts (delegates or fallback).
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if e.inner != nil {
		resp, err := e.inner.Embed(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("openai embedding batch: %w", err)
		}
		if len(resp.Data) != len(texts) {
			return nil, fmt.Errorf("openai embedding batch: expected %d results, got %d", len(texts), len(resp.Data))
		}
		out := make([][]float32, len(resp.Data))
		for i, d := range resp.Data {
			out[i] = d.Embedding
		}
		return out, nil
	}
	// fallback
	if e.client == nil {
		return nil, fmt.Errorf("openai embedding batch: no client")
	}
	resp, err := e.client.CreateEmbeddings(ctx, goopenai.EmbeddingRequestStrings{
		Input: texts,
		Model: goopenai.EmbeddingModel(e.modelName),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embedding batch: %w", err)
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("openai embedding batch: expected %d results, got %d", len(texts), len(resp.Data))
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

// AsModel wraps as model.EmbeddingModel (uses existing adapter).
func (e *OpenAIEmbedder) AsModel(dimensions int) model.EmbeddingModel {
	if e == nil {
		return nil
	}
	return modelembed.FromMemory(e, e.inner.ModelName(), dimensions)
}

var _ memory.EmbeddingModel = (*OpenAIEmbedder)(nil)
