package model

import "context"

// EmbeddingUsage reports token usage for an embedding call.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// EmbeddingData is a single embedding vector in an API response.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// EmbeddingResponse mirrors OpenAI-compatible embedding responses.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage,omitempty"`
}

// EmbeddingModel generates vector embeddings for text inputs.
type EmbeddingModel interface {
	ModelName() string
	Dimensions() int
	Embed(ctx context.Context, input []string) (*EmbeddingResponse, error)
}
