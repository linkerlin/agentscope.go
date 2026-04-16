package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/retry"
)

// OpenAIChatModel implements model.ChatModel using the OpenAI API
type OpenAIChatModel struct {
	client           *goopenai.Client
	modelName        string
	retryMaxAttempts int
	retryBackoff     time.Duration
	formatter        *formatter.OpenAIFormatter
}

// OpenAIChatModelBuilder builds an OpenAIChatModel
type OpenAIChatModelBuilder struct {
	apiKey           string
	modelName        string
	baseURL          string
	retryMaxAttempts int
	retryBackoff     time.Duration
	fmt              *formatter.OpenAIFormatter
}

// Builder returns a new OpenAIChatModelBuilder
func Builder() *OpenAIChatModelBuilder {
	return &OpenAIChatModelBuilder{
		modelName: goopenai.GPT4oMini,
	}
}

func (b *OpenAIChatModelBuilder) APIKey(key string) *OpenAIChatModelBuilder {
	b.apiKey = key
	return b
}

func (b *OpenAIChatModelBuilder) ModelName(name string) *OpenAIChatModelBuilder {
	b.modelName = name
	return b
}

func (b *OpenAIChatModelBuilder) BaseURL(url string) *OpenAIChatModelBuilder {
	b.baseURL = url
	return b
}

// Retry 设置 Chat/ChatStream 建立连接前的重试策略；maxAttempts < 2 表示关闭（默认关闭）
func (b *OpenAIChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *OpenAIChatModelBuilder {
	b.retryMaxAttempts = maxAttempts
	b.retryBackoff = backoff
	return b
}

// Formatter sets a custom formatter (defaults to formatter.NewOpenAIFormatter())
func (b *OpenAIChatModelBuilder) Formatter(f *formatter.OpenAIFormatter) *OpenAIChatModelBuilder {
	b.fmt = f
	return b
}

func (b *OpenAIChatModelBuilder) Build() (*OpenAIChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("openai: API key is required")
	}
	cfg := goopenai.DefaultConfig(b.apiKey)
	if b.baseURL != "" {
		cfg.BaseURL = b.baseURL
	}
	f := b.fmt
	if f == nil {
		f = formatter.NewOpenAIFormatter()
	}
	return &OpenAIChatModel{
		client:           goopenai.NewClientWithConfig(cfg),
		modelName:        b.modelName,
		retryMaxAttempts: b.retryMaxAttempts,
		retryBackoff:     b.retryBackoff,
		formatter:        f,
	}, nil
}

func (m *OpenAIChatModel) ModelName() string { return m.modelName }

// Chat converts Msg slice to OpenAI messages, calls the API, and converts back
func (m *OpenAIChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.retryMaxAttempts < 2 {
		return m.chatOnce(ctx, messages, options...)
	}
	ro := retry.Options{MaxAttempts: m.retryMaxAttempts, Backoff: m.retryBackoff}
	var out *message.Msg
	err := retry.Do(ctx, ro, func() error {
		msg, err := m.chatOnce(ctx, messages, options...)
		if err != nil {
			return classifyOpenAIErr(err)
		}
		out = msg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *OpenAIChatModel) chatOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	opts := applyOptions(options)
	req := goopenai.ChatCompletionRequest{
		Model:    m.modelName,
		Messages: m.formatter.FormatMessagesTyped(messages),
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		req.Temperature = float32(opts.Temperature)
	}
	if len(opts.Tools) > 0 {
		req.Tools = m.formatter.FormatToolsTyped(opts.Tools)
	}
	if opts.ToolChoice != nil {
		req.ToolChoice, _ = m.formatter.FormatToolChoice(opts.ToolChoice)
	}

	resp, err := m.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai chat: no choices returned")
	}
	msg := m.formatter.ParseChoice(resp.Choices[0])
	if resp.Usage.TotalTokens > 0 {
		msg.Metadata["usage"] = model.ChatUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return msg, nil
}

// ChatStream calls the OpenAI streaming API and returns a channel of StreamChunks
func (m *OpenAIChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	if m.retryMaxAttempts < 2 {
		return m.chatStreamOnce(ctx, messages, options...)
	}
	ro := retry.Options{MaxAttempts: m.retryMaxAttempts, Backoff: m.retryBackoff}
	var out <-chan *model.StreamChunk
	err := retry.Do(ctx, ro, func() error {
		ch, err := m.chatStreamOnce(ctx, messages, options...)
		if err != nil {
			return classifyOpenAIErr(err)
		}
		out = ch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *OpenAIChatModel) chatStreamOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	opts := applyOptions(options)
	req := goopenai.ChatCompletionRequest{
		Model:    m.modelName,
		Messages: m.formatter.FormatMessagesTyped(messages),
		Stream:   true,
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		req.Temperature = float32(opts.Temperature)
	}
	if len(opts.Tools) > 0 {
		req.Tools = m.formatter.FormatToolsTyped(opts.Tools)
	}
	if opts.ToolChoice != nil {
		req.ToolChoice, _ = m.formatter.FormatToolChoice(opts.ToolChoice)
	}

	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	ch := make(chan *model.StreamChunk, 64)
	go func() {
		defer close(ch)
		defer stream.Close()
		var usage model.ChatUsage
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				ch <- &model.StreamChunk{Done: true, Usage: &usage}
				return
			}
			if err != nil {
				ch <- &model.StreamChunk{Done: true, Usage: &usage}
				return
			}
			if len(resp.Choices) == 0 {
				continue
			}
			delta := resp.Choices[0].Delta.Content
			if resp.Usage.TotalTokens > 0 {
				usage = model.ChatUsage{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      resp.Usage.TotalTokens,
				}
			}
			ch <- &model.StreamChunk{Delta: delta}
		}
	}()
	return ch, nil
}

// ---- helpers ----

func applyOptions(options []model.ChatOption) *model.ChatOptions {
	opts := &model.ChatOptions{}
	for _, o := range options {
		o(opts)
	}
	return opts
}


