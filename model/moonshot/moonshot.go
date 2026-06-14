// Package moonshot provides a ChatModel implementation for the Moonshot (Kimi) API.
// Moonshot's API is OpenAI-compatible, so this package reuses the openai
// implementation with a different base URL and default model.
package moonshot

import (
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/model/openai"
)

// Default base URL for Moonshot API.
const DefaultBaseURL = "https://api.moonshot.cn/v1"

// Default model names supported by Moonshot.
const (
	Model8K    = "moonshot-v1-8k"
	Model32K   = "moonshot-v1-32k"
	Model128K  = "moonshot-v1-128k"
	ModelLight = "moonshot-v1-8k-20240416" // lightweight version
)

// MoonshotChatModelBuilder builds a MoonshotChatModel.
// It is a thin wrapper over openai.OpenAIChatModelBuilder that sets the
// Moonshot base URL and sensible defaults.
type MoonshotChatModelBuilder struct {
	inner *openai.OpenAIChatModelBuilder
}

// Builder returns a new MoonshotChatModelBuilder.
func Builder(apiKey string) *MoonshotChatModelBuilder {
	return &MoonshotChatModelBuilder{
		inner: openai.Builder().
			APIKey(apiKey).
			BaseURL(DefaultBaseURL).
			ModelName(Model8K),
	}
}

// NewBuilder is an alias for Builder, following the Go New-prefix convention.
func NewBuilder(apiKey string) *MoonshotChatModelBuilder { return Builder(apiKey) }

// APIKey sets the Moonshot API key.
func (b *MoonshotChatModelBuilder) APIKey(key string) *MoonshotChatModelBuilder {
	b.inner.APIKey(key)
	return b
}

// ModelName sets the model name (e.g., moonshot-v1-8k, moonshot-v1-32k, moonshot-v1-128k).
func (b *MoonshotChatModelBuilder) ModelName(name string) *MoonshotChatModelBuilder {
	b.inner.ModelName(name)
	return b
}

// BaseURL overrides the default Moonshot base URL (for proxies or on-prem deployments).
func (b *MoonshotChatModelBuilder) BaseURL(url string) *MoonshotChatModelBuilder {
	b.inner.BaseURL(url)
	return b
}

// Retry configures connection-level retry policy.
func (b *MoonshotChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *MoonshotChatModelBuilder {
	b.inner.Retry(maxAttempts, backoff)
	return b
}

// Formatter sets a custom formatter (defaults to OpenAI-compatible formatter).
func (b *MoonshotChatModelBuilder) Formatter(f *formatter.OpenAIFormatter) *MoonshotChatModelBuilder {
	b.inner.Formatter(f)
	return b
}

// Build constructs the MoonshotChatModel.
func (b *MoonshotChatModelBuilder) Build() (*openai.OpenAIChatModel, error) {
	return b.inner.Build()
}
