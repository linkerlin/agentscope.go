package embedding

import (
	"github.com/linkerlin/agentscope.go/model"
	modelembed "github.com/linkerlin/agentscope.go/model/embedding"
)

// NewOllama creates an embedding model that talks to a local Ollama server
// (uses Ollama's native /api/embed for best compatibility).
func NewOllama(baseURL, modelName string, dimension int) model.EmbeddingModel {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return modelembed.NewOllamaEmbedder(baseURL, modelName, dimension)
}
