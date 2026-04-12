package dashscope

import (
	"context"
	"errors"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	oai "github.com/linkerlin/agentscope.go/model/openai"
)

const defaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

// DashScopeChatModel wraps OpenAIChatModel with DashScope defaults
type DashScopeChatModel struct {
	inner *oai.OpenAIChatModel
}

// DashScopeChatModelBuilder builds a DashScopeChatModel
type DashScopeChatModelBuilder struct {
	apiKey           string
	modelName        string
	baseURL          string
	retryMaxAttempts int
	retryBackoff     time.Duration
}

// Builder returns a new DashScopeChatModelBuilder
func Builder() *DashScopeChatModelBuilder {
	return &DashScopeChatModelBuilder{
		modelName: "qwen-plus",
		baseURL:   defaultBaseURL,
	}
}

func (b *DashScopeChatModelBuilder) APIKey(key string) *DashScopeChatModelBuilder {
	b.apiKey = key
	return b
}

func (b *DashScopeChatModelBuilder) ModelName(name string) *DashScopeChatModelBuilder {
	b.modelName = name
	return b
}

func (b *DashScopeChatModelBuilder) BaseURL(url string) *DashScopeChatModelBuilder {
	b.baseURL = url
	return b
}

// Retry 透传至底层 OpenAI 兼容客户端（maxAttempts < 2 关闭）
func (b *DashScopeChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *DashScopeChatModelBuilder {
	b.retryMaxAttempts = maxAttempts
	b.retryBackoff = backoff
	return b
}

func (b *DashScopeChatModelBuilder) Build() (*DashScopeChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("dashscope: API key is required")
	}
	inner, err := oai.Builder().
		APIKey(b.apiKey).
		ModelName(b.modelName).
		BaseURL(b.baseURL).
		Retry(b.retryMaxAttempts, b.retryBackoff).
		Build()
	if err != nil {
		return nil, err
	}
	return &DashScopeChatModel{inner: inner}, nil
}

func (m *DashScopeChatModel) ModelName() string { return m.inner.ModelName() }

func (m *DashScopeChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return m.inner.Chat(ctx, messages, options...)
}

func (m *DashScopeChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return m.inner.ChatStream(ctx, messages, options...)
}
