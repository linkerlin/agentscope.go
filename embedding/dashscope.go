package embedding

import (
	"github.com/linkerlin/agentscope.go/model"
)

// NewDashScope creates a DashScope embedding model using the OpenAI-compatible endpoint.
// modelName defaults to "text-embedding-v1" (use "multimodal-embedding-v1" for multimodal text+image support).
// The compat endpoint handles basic; for advanced multimodal DashScope specific API, can extend like Gemini impl.
// Use NewOpenAIWithBaseURL under the hood for compatibility (reduces code dup).
func NewDashScope(apiKey, modelName string) model.EmbeddingModel {
	if modelName == "" {
		modelName = "text-embedding-v1"
	}
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"
	// For multimodal DashScope, user can pass "multimodal-embedding-v1" and appropriate input handling (future).
	return NewOpenAIWithBaseURL(apiKey, baseURL, modelName)
}

// Note: For full multimodal (image+text) DashScope, extend the input handling in future.
// The compatible endpoint supports text; for advanced, direct HTTP to DashScope embedding API can be added similar to Gemini.
