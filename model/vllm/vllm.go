// Package vllm provides a ChatModel implementation for vLLM OpenAI-compatible API.
// vLLM is a high-throughput and memory-efficient inference engine for LLMs,
// commonly used for self-hosted / on-premise deployments.
package vllm

import (
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

// DefaultBaseURL is the standard vLLM OpenAI-compatible endpoint.
const DefaultBaseURL = "http://localhost:8000/v1"

// VLLMChatModelBuilder builds a ChatModel for vLLM.
// It is a thin wrapper over openai.OpenAIChatModelBuilder that sets the
// vLLM base URL and sensible defaults.
type VLLMChatModelBuilder struct {
	inner *openai.OpenAIChatModelBuilder
}

// Builder returns a new VLLMChatModelBuilder.
func Builder(apiKey string) *VLLMChatModelBuilder {
	return &VLLMChatModelBuilder{
		inner: openai.Builder().
			APIKey(apiKey).
			BaseURL(DefaultBaseURL).
			ModelName(""),
	}
}

// NewBuilder is an alias for Builder, following the Go New-prefix convention.
func NewBuilder(apiKey string) *VLLMChatModelBuilder { return Builder(apiKey) }

// APIKey sets the API key (vLLM usually accepts "not-needed" or empty).
func (b *VLLMChatModelBuilder) APIKey(key string) *VLLMChatModelBuilder {
	b.inner.APIKey(key)
	return b
}

// ModelName sets the model name (optional for vLLM; inferred from served model).
func (b *VLLMChatModelBuilder) ModelName(name string) *VLLMChatModelBuilder {
	b.inner.ModelName(name)
	return b
}

// BaseURL overrides the default vLLM base URL.
func (b *VLLMChatModelBuilder) BaseURL(url string) *VLLMChatModelBuilder {
	b.inner.BaseURL(url)
	return b
}

// Retry configures connection-level retry policy.
func (b *VLLMChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *VLLMChatModelBuilder {
	b.inner.Retry(maxAttempts, backoff)
	return b
}

// Formatter sets a custom formatter (defaults to OpenAI-compatible formatter).
func (b *VLLMChatModelBuilder) Formatter(f *formatter.OpenAIFormatter) *VLLMChatModelBuilder {
	b.inner.Formatter(f)
	return b
}

// Build constructs the vLLM ChatModel.
func (b *VLLMChatModelBuilder) Build() (*openai.OpenAIChatModel, error) {
	return b.inner.Build()
}
