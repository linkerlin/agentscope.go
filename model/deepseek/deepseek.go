// Package deepseek provides a ChatModel implementation for the DeepSeek API.
// DeepSeek's API is OpenAI-compatible, so this package reuses the openai
// implementation with a different base URL and default model.
package deepseek

import (
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

// Default base URL for DeepSeek API.
const DefaultBaseURL = "https://api.deepseek.com/v1"

// Default model names supported by DeepSeek.
const (
	ModelChat   = "deepseek-chat"
	ModelCoder  = "deepseek-coder"
	ModelReason = "deepseek-reasoner"
)

// DeepSeekChatModelBuilder builds a DeepSeekChatModel.
// It is a thin wrapper over openai.OpenAIChatModelBuilder that sets the
// DeepSeek base URL and sensible defaults.
type DeepSeekChatModelBuilder struct {
	inner *openai.OpenAIChatModelBuilder
}

// Builder returns a new DeepSeekChatModelBuilder.
func Builder(apiKey string) *DeepSeekChatModelBuilder {
	return &DeepSeekChatModelBuilder{
		inner: openai.Builder().
			APIKey(apiKey).
			BaseURL(DefaultBaseURL).
			ModelName(ModelChat),
	}
}

// APIKey sets the DeepSeek API key.
func (b *DeepSeekChatModelBuilder) APIKey(key string) *DeepSeekChatModelBuilder {
	b.inner.APIKey(key)
	return b
}

// ModelName sets the model name (e.g., deepseek-chat, deepseek-coder, deepseek-reasoner).
func (b *DeepSeekChatModelBuilder) ModelName(name string) *DeepSeekChatModelBuilder {
	b.inner.ModelName(name)
	return b
}

// BaseURL overrides the default DeepSeek base URL (for proxies or on-prem deployments).
func (b *DeepSeekChatModelBuilder) BaseURL(url string) *DeepSeekChatModelBuilder {
	b.inner.BaseURL(url)
	return b
}

// Retry configures connection-level retry policy.
func (b *DeepSeekChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *DeepSeekChatModelBuilder {
	b.inner.Retry(maxAttempts, backoff)
	return b
}

// Formatter sets a custom formatter (defaults to OpenAI-compatible formatter).
func (b *DeepSeekChatModelBuilder) Formatter(f *formatter.OpenAIFormatter) *DeepSeekChatModelBuilder {
	b.inner.Formatter(f)
	return b
}

// Build constructs the DeepSeekChatModel.
func (b *DeepSeekChatModelBuilder) Build() (*openai.OpenAIChatModel, error) {
	return b.inner.Build()
}
