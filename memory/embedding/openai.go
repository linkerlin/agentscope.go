package embedding

import (
	"context"
	"fmt"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/memory"
)

// OpenAIEmbedder uses the OpenAI API to generate text embeddings.
type OpenAIEmbedder struct {
	client    *goopenai.Client
	modelName string
}

// NewOpenAIEmbedder creates an embedder with the given API key and model.
// If modelName is empty, it defaults to text-embedding-3-small.
func NewOpenAIEmbedder(apiKey, modelName string) *OpenAIEmbedder {
	if modelName == "" {
		modelName = string(goopenai.SmallEmbedding3)
	}
	return &OpenAIEmbedder{
		client:    goopenai.NewClient(apiKey),
		modelName: modelName,
	}
}

// NewOpenAIEmbedderWithBaseURL creates an embedder with a custom base URL
// (useful for OpenAI-compatible services such as Ollama, vLLM, etc.).
func NewOpenAIEmbedderWithBaseURL(apiKey, baseURL, modelName string) *OpenAIEmbedder {
	if modelName == "" {
		modelName = string(goopenai.SmallEmbedding3)
	}
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAIEmbedder{
		client:    goopenai.NewClientWithConfig(cfg),
		modelName: modelName,
	}
}

// Embed generates an embedding vector for a single text.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
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

// EmbedBatch generates embedding vectors for multiple texts in one request.
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
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

var _ memory.EmbeddingModel = (*OpenAIEmbedder)(nil)
