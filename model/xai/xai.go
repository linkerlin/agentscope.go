// Package xai provides a ChatModel implementation for the xAI (Grok) API.
// xAI's API is OpenAI-compatible, so this package reuses the openai
// implementation with a different base URL and default model.
package xai

import (
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

// Default base URL for xAI API.
const DefaultBaseURL = "https://api.x.ai/v1"

// Default model names supported by xAI.
const (
	ModelGrokBeta        = "grok-beta"
	ModelGrok2           = "grok-2"
	ModelGrok2Vision     = "grok-2-vision-1212"
	ModelGrok2Mini       = "grok-2-1212"
	ModelGrokVisionBeta  = "grok-vision-beta"
)

// XAIChatModelBuilder builds an XAIChatModel.
// It is a thin wrapper over openai.OpenAIChatModelBuilder that sets the
// xAI base URL and sensible defaults.
type XAIChatModelBuilder struct {
	inner *openai.OpenAIChatModelBuilder
}

// Builder returns a new XAIChatModelBuilder.
func Builder(apiKey string) *XAIChatModelBuilder {
	return &XAIChatModelBuilder{
		inner: openai.Builder().
			APIKey(apiKey).
			BaseURL(DefaultBaseURL).
			ModelName(ModelGrokBeta),
	}
}

// APIKey sets the xAI API key.
func (b *XAIChatModelBuilder) APIKey(key string) *XAIChatModelBuilder {
	b.inner.APIKey(key)
	return b
}

// ModelName sets the model name (e.g., grok-beta, grok-2, grok-2-vision-1212).
func (b *XAIChatModelBuilder) ModelName(name string) *XAIChatModelBuilder {
	b.inner.ModelName(name)
	return b
}

// BaseURL overrides the default xAI base URL (for proxies or on-prem deployments).
func (b *XAIChatModelBuilder) BaseURL(url string) *XAIChatModelBuilder {
	b.inner.BaseURL(url)
	return b
}

// Retry configures connection-level retry policy.
func (b *XAIChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *XAIChatModelBuilder {
	b.inner.Retry(maxAttempts, backoff)
	return b
}

// Formatter sets a custom formatter (defaults to OpenAI-compatible formatter).
func (b *XAIChatModelBuilder) Formatter(f *formatter.OpenAIFormatter) *XAIChatModelBuilder {
	b.inner.Formatter(f)
	return b
}

// Build constructs the XAIChatModel.
func (b *XAIChatModelBuilder) Build() (*openai.OpenAIChatModel, error) {
	return b.inner.Build()
}
