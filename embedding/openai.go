package embedding

import (
	"context"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/model"
)

// NewOpenAI creates an OpenAI (or compatible) embedding model.
// modelName defaults to "text-embedding-3-small" if empty.
func NewOpenAI(apiKey, modelName string) model.EmbeddingModel {
	if modelName == "" {
		modelName = string(goopenai.SmallEmbedding3)
	}
	client := goopenai.NewClient(apiKey)
	return &openaiModel{
		client:    client,
		modelName: modelName,
	}
}

// NewOpenAIWithBaseURL is useful for OpenAI-compatible servers (Ollama, vLLM, etc.).
func NewOpenAIWithBaseURL(apiKey, baseURL, modelName string) model.EmbeddingModel {
	if modelName == "" {
		modelName = string(goopenai.SmallEmbedding3)
	}
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := goopenai.NewClientWithConfig(cfg)
	return &openaiModel{
		client:    client,
		modelName: modelName,
	}
}

type openaiModel struct {
	client    *goopenai.Client
	modelName string
}

func (m *openaiModel) ModelName() string { return m.modelName }
func (m *openaiModel) Dimensions() int {
	// Reasonable defaults; real value can be queried from model cards in practice.
	switch m.modelName {
	case string(goopenai.SmallEmbedding3):
		return 1536
	case string(goopenai.LargeEmbedding3):
		return 3072
	case string(goopenai.AdaEmbeddingV2):
		return 1536
	default:
		// fallback for custom or other models
		return 1536
	}
}

func (m *openaiModel) Embed(ctx context.Context, input []string) (*model.EmbeddingResponse, error) {
	if len(input) == 0 {
		return &model.EmbeddingResponse{Object: "list", Model: m.modelName}, nil
	}

	req := goopenai.EmbeddingRequestStrings{
		Input: input,
		Model: goopenai.EmbeddingModel(m.modelName),
	}

	resp, err := m.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}

	out := &model.EmbeddingResponse{
		Object: "list",
		Model:  m.modelName,
		Usage: model.EmbeddingUsage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}
	for i, item := range resp.Data {
		out.Data = append(out.Data, model.EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: item.Embedding,
		})
	}
	return out, nil
}

var _ model.EmbeddingModel = (*openaiModel)(nil)
