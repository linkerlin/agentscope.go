package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/retry"
)

const defaultBaseURL = "https://api.anthropic.com/v1"
const apiVersion = "2023-06-01"

// ChatModel implements model.ChatModel for the Anthropic Messages API.
type ChatModel struct {
	client           *http.Client
	apiKey           string
	baseURL          string
	modelName        string
	maxTokens        int
	retryMaxAttempts int
	retryBackoff     time.Duration
	fmt              *formatter.AnthropicFormatter
}

// Builder constructs a ChatModel.
type Builder struct {
	apiKey           string
	modelName        string
	baseURL          string
	maxTokens        int
	retryMaxAttempts int
	retryBackoff     time.Duration
	fmt              *formatter.AnthropicFormatter
}

// NewBuilder returns a new Builder with defaults.
func NewBuilder() *Builder {
	return &Builder{
		modelName: "claude-3-5-sonnet-20241022",
		baseURL:   defaultBaseURL,
		maxTokens: 4096,
	}
}

func (b *Builder) APIKey(key string) *Builder {
	b.apiKey = key
	return b
}

func (b *Builder) ModelName(name string) *Builder {
	b.modelName = name
	return b
}

func (b *Builder) BaseURL(url string) *Builder {
	b.baseURL = url
	return b
}

func (b *Builder) MaxTokens(n int) *Builder {
	b.maxTokens = n
	return b
}

func (b *Builder) Retry(maxAttempts int, backoff time.Duration) *Builder {
	b.retryMaxAttempts = maxAttempts
	b.retryBackoff = backoff
	return b
}

func (b *Builder) Formatter(f *formatter.AnthropicFormatter) *Builder {
	b.fmt = f
	return b
}

func (b *Builder) Build() (*ChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("anthropic: API key is required")
	}
	f := b.fmt
	if f == nil {
		f = formatter.NewAnthropicFormatter()
	}
	return &ChatModel{
		client:           &http.Client{Timeout: 120 * time.Second},
		apiKey:           b.apiKey,
		baseURL:          b.baseURL,
		modelName:        b.modelName,
		maxTokens:        b.maxTokens,
		retryMaxAttempts: b.retryMaxAttempts,
		retryBackoff:     b.retryBackoff,
		fmt:              f,
	}, nil
}

func (m *ChatModel) ModelName() string { return m.modelName }

// Chat performs a non-streaming chat request.
func (m *ChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.retryMaxAttempts < 2 {
		return m.chatOnce(ctx, messages, options...)
	}
	ro := retry.Options{MaxAttempts: m.retryMaxAttempts, Backoff: m.retryBackoff}
	var out *message.Msg
	err := retry.Do(ctx, ro, func() error {
		msg, err := m.chatOnce(ctx, messages, options...)
		if err != nil {
			return err
		}
		out = msg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *ChatModel) chatOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	body, err := m.buildRequestBody(messages, false, options...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic chat: %s %s", resp.Status, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}
	return m.fmt.ParseResponse(result)
}

// ChatStream performs a streaming chat request via SSE.
func (m *ChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	if m.retryMaxAttempts < 2 {
		return m.chatStreamOnce(ctx, messages, options...)
	}
	ro := retry.Options{MaxAttempts: m.retryMaxAttempts, Backoff: m.retryBackoff}
	var out <-chan *model.StreamChunk
	err := retry.Do(ctx, ro, func() error {
		ch, err := m.chatStreamOnce(ctx, messages, options...)
		if err != nil {
			return err
		}
		out = ch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *ChatModel) chatStreamOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	body, err := m.buildRequestBody(messages, true, options...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic stream: %s %s", resp.Status, string(respBody))
	}

	ch := make(chan *model.StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		var usage model.ChatUsage
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- &model.StreamChunk{Done: true, Usage: &usage}
				return
			}
			var ev map[string]any
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			if ev["type"] == "message_delta" {
				if u, ok := ev["usage"].(map[string]any); ok {
					usage.CompletionTokens = intAny(u["output_tokens"])
				}
			}
			if ev["type"] != "content_block_delta" {
				continue
			}
			delta, ok := ev["delta"].(map[string]any)
			if !ok {
				continue
			}
			text, _ := delta["text"].(string)
			if text != "" {
				ch <- &model.StreamChunk{Delta: text}
			}
			if thinking, ok := delta["thinking"].(string); ok && thinking != "" {
				ch <- &model.StreamChunk{Content: []message.ContentBlock{message.NewThinkingBlock(thinking, "")}}
			}
		}
		ch <- &model.StreamChunk{Done: true, Usage: &usage}
	}()
	return ch, nil
}

func (m *ChatModel) buildRequestBody(messages []*message.Msg, stream bool, options ...model.ChatOption) ([]byte, error) {
	opts := &model.ChatOptions{}
	for _, o := range options {
		o(opts)
	}
	amsgs, system := m.fmt.FormatMessages(messages)

	req := map[string]any{
		"model":      m.modelName,
		"max_tokens": m.maxTokens,
		"messages":   amsgs,
		"stream":     stream,
	}
	if system != "" {
		req["system"] = system
	}
	if opts.Temperature > 0 {
		req["temperature"] = opts.Temperature
	}
	if len(opts.Tools) > 0 {
		req["tools"] = m.fmt.FormatTools(opts.Tools)
	}
	if opts.ToolChoice != nil {
		req["tool_choice"] = m.fmt.FormatToolChoice(opts.ToolChoice)
	}
	return json.Marshal(req)
}

func intAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

var _ model.ChatModel = (*ChatModel)(nil)
