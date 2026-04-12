package ollama

import (
	"context"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	oai "github.com/linkerlin/agentscope.go/model/openai"
)

// 默认 Ollama OpenAI 兼容端点
const defaultBaseURL = "http://127.0.0.1:11434/v1"

// ChatModel 通过 OpenAI 兼容 HTTP API 连接 Ollama
type ChatModel struct {
	inner *oai.OpenAIChatModel
}

// Builder 构建 Ollama ChatModel
type Builder struct {
	modelName        string
	baseURL          string
	apiKey           string
	retryMaxAttempts int
	retryBackoff     time.Duration
}

// NewBuilder 创建构建器，默认模型 llama3.2
func NewBuilder() *Builder {
	return &Builder{
		modelName: "llama3.2",
		baseURL:   defaultBaseURL,
		apiKey:    "ollama",
	}
}

func (b *Builder) ModelName(name string) *Builder {
	b.modelName = name
	return b
}

func (b *Builder) BaseURL(url string) *Builder {
	b.baseURL = url
	return b
}

// APIKey 兼容层常需非空占位
func (b *Builder) APIKey(key string) *Builder {
	b.apiKey = key
	return b
}

// Retry 透传底层 openai 客户端
func (b *Builder) Retry(maxAttempts int, backoff time.Duration) *Builder {
	b.retryMaxAttempts = maxAttempts
	b.retryBackoff = backoff
	return b
}

// Build 构建模型
func (b *Builder) Build() (*ChatModel, error) {
	if b.apiKey == "" {
		b.apiKey = "ollama"
	}
	ob := oai.Builder().APIKey(b.apiKey).ModelName(b.modelName).BaseURL(b.baseURL).Retry(b.retryMaxAttempts, b.retryBackoff)
	inner, err := ob.Build()
	if err != nil {
		return nil, err
	}
	return &ChatModel{inner: inner}, nil
}

func (m *ChatModel) ModelName() string { return m.inner.ModelName() }

func (m *ChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return m.inner.Chat(ctx, messages, options...)
}

func (m *ChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return m.inner.ChatStream(ctx, messages, options...)
}

var _ model.ChatModel = (*ChatModel)(nil)
