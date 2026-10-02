// Package ark provides a ChatModel implementation for Volcano Ark (火山方舟,
// ByteDance) — Doubao chat models (doubao-seed-*) over the OpenAI-compatible
// /api/v3/chat/completions endpoint, implemented directly on net/http (same
// policy as openai_response): Ark-specific request knobs (the `thinking`
// switch) and response fields (`reasoning_content`) need full wire control.
//
// The wire types and the "no content block is dropped" parsing live in
// formatter/ark.go; this package is the transport.
package ark

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

// DefaultBaseURL is the Volcano Ark OpenAI-compatible endpoint.
const DefaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

// Default Doubao model names.
const (
	ModelDoubaoSeed = "doubao-seed-1-6" // thinking-capable flagship line
)

// ArkChatModel implements model.ChatModel for Volcano Ark.
type ArkChatModel struct {
	httpClient   *http.Client
	apiKey       string
	baseURL      string
	modelName    string
	thinking     string // "enabled" | "disabled" | "" (server default)
	retryMax     int
	retryBackoff time.Duration
	fmt          *formatter.ArkFormatter
}

// Builder returns a new ArkChatModelBuilder.
func Builder(apiKey string) *ArkChatModelBuilder {
	return &ArkChatModelBuilder{
		apiKey:    apiKey,
		baseURL:   DefaultBaseURL,
		modelName: ModelDoubaoSeed,
		http:      &http.Client{Timeout: 120 * time.Second},
	}
}

// NewBuilder is an alias for Builder, following the Go New-prefix convention.
func NewBuilder(apiKey string) *ArkChatModelBuilder { return Builder(apiKey) }

// ArkChatModelBuilder builds an ArkChatModel.
type ArkChatModelBuilder struct {
	apiKey    string
	baseURL   string
	modelName string
	thinking  string
	http      *http.Client
	retryMax  int
	backoff   time.Duration
	fmt       *formatter.ArkFormatter
}

// BaseURL overrides the API base URL (proxies, tests).
func (b *ArkChatModelBuilder) BaseURL(u string) *ArkChatModelBuilder {
	b.baseURL = u
	return b
}

// ModelName sets the Doubao model (e.g. doubao-seed-1-6).
func (b *ArkChatModelBuilder) ModelName(m string) *ArkChatModelBuilder {
	b.modelName = m
	return b
}

// Thinking enables or disables the doubao-seed thinking mode (the Ark
// `thinking` request parameter). Unset = server default.
func (b *ArkChatModelBuilder) Thinking(enabled bool) *ArkChatModelBuilder {
	if enabled {
		b.thinking = "enabled"
	} else {
		b.thinking = "disabled"
	}
	return b
}

// HTTPClient sets a custom HTTP client (tests inject mocks).
func (b *ArkChatModelBuilder) HTTPClient(c *http.Client) *ArkChatModelBuilder {
	b.http = c
	return b
}

// Retry configures connection-level retry policy.
func (b *ArkChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *ArkChatModelBuilder {
	b.retryMax, b.backoff = maxAttempts, backoff
	return b
}

// Formatter overrides the default ArkFormatter.
func (b *ArkChatModelBuilder) Formatter(f *formatter.ArkFormatter) *ArkChatModelBuilder {
	b.fmt = f
	return b
}

// Build constructs the ArkChatModel.
func (b *ArkChatModelBuilder) Build() (*ArkChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("ark: API key is required")
	}
	f := b.fmt
	if f == nil {
		f = formatter.NewArkFormatter()
	}
	httpClient := b.http
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &ArkChatModel{
		httpClient:   httpClient,
		apiKey:       b.apiKey,
		baseURL:      b.baseURL,
		modelName:    b.modelName,
		thinking:     b.thinking,
		retryMax:     b.retryMax,
		retryBackoff: b.backoff,
		fmt:          f,
	}, nil
}

// ModelName implements model.ChatModel.
func (m *ArkChatModel) ModelName() string { return m.modelName }

// Chat implements model.ChatModel.
func (m *ArkChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.retryMax >= 2 {
		var out *message.Msg
		ro := retry.Options{MaxAttempts: m.retryMax, Backoff: m.retryBackoff}
		err := retry.Do(ctx, ro, func() error {
			msg, err := m.chatOnce(ctx, messages, options...)
			if err != nil {
				return err // Ark errors are classified below
			}
			out = msg
			return nil
		})
		return out, err
	}
	return m.chatOnce(ctx, messages, options...)
}

func (m *ArkChatModel) chatOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	body, err := m.buildRequest(messages, options)
	if err != nil {
		return nil, err
	}
	raw, err := m.postJSON(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp formatter.ArkChatResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("ark: decode response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("ark: %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("ark: empty choices in response")
	}
	return m.fmt.ParseArkMessage(&resp.Choices[0].Message), nil
}

// ChatStream implements model.ChatModel (SSE).
func (m *ArkChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	body, err := m.buildRequest(messages, options)
	if err != nil {
		return nil, err
	}
	body["stream"] = true
	body["stream_options"] = map[string]any{"include_usage": true}
	raw, err := m.postStream(ctx, body)
	if err != nil {
		return nil, err
	}

	out := make(chan *model.StreamChunk, 16)
	go func() {
		defer close(out)
		defer raw.Close()
		scanner := bufio.NewScanner(raw)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var ev formatter.ArkStreamEvent
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue // forward-compatible: unknown frames skipped
			}
			if ev.Error != nil {
				out <- &model.StreamChunk{Done: true, Delta: fmt.Sprintf("ark stream error: %s", ev.Error.Message)}
				return
			}
			for _, ch := range m.fmt.ParseArkStreamDelta(&ev) {
				out <- ch
			}
			if ev.Usage != nil {
				out <- &model.StreamChunk{
					Done: true,
					Usage: &model.ChatUsage{
						PromptTokens:     ev.Usage.PromptTokens,
						CompletionTokens: ev.Usage.CompletionTokens,
						TotalTokens:      ev.Usage.TotalTokens,
					},
				}
				return
			}
		}
		out <- &model.StreamChunk{Done: true}
	}()
	return out, nil
}

// buildRequest assembles the OpenAI-compatible request body plus the Ark
// `thinking` switch.
func (m *ArkChatModel) buildRequest(messages []*message.Msg, options []model.ChatOption) (map[string]any, error) {
	opts := model.ApplyOptions(options)
	typed := m.fmt.FormatMessagesTyped(messages)
	msgsRaw, err := json.Marshal(typed)
	if err != nil {
		return nil, fmt.Errorf("ark: marshal messages: %w", err)
	}
	var msgs []any
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return nil, fmt.Errorf("ark: re-message messages: %w", err)
	}
	body := map[string]any{
		"model":    m.modelName,
		"messages": msgs,
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}
	if m.thinking != "" {
		body["thinking"] = map[string]any{"type": m.thinking}
	}
	if len(opts.Tools) > 0 {
		toolsRaw, err := json.Marshal(m.fmt.FormatToolsTyped(opts.Tools))
		if err != nil {
			return nil, fmt.Errorf("ark: marshal tools: %w", err)
		}
		var tools []any
		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return nil, fmt.Errorf("ark: re-message tools: %w", err)
		}
		body["tools"] = tools
	}
	if opts.ToolChoice != nil {
		tc, err := m.fmt.FormatToolChoice(opts.ToolChoice)
		if err == nil && tc != nil {
			body["tool_choice"] = tc
		}
	}
	if opts.ResponseFormat != nil {
		body["response_format"] = formatter.ArkResponseFormat(opts.ResponseFormat)
	}
	return body, nil
}

// postJSON sends the request and returns the full response body.
func (m *ArkChatModel) postJSON(ctx context.Context, body map[string]any) ([]byte, error) {
	resp, err := m.doPost(ctx, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ark: read response: %w", err)
	}
	return data, nil
}

// postStream sends the request and returns the raw SSE body stream.
func (m *ArkChatModel) postStream(ctx context.Context, body map[string]any) (io.ReadCloser, error) {
	resp, err := m.doPost(ctx, body)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// doPost sends one request; non-200 responses are decoded into a typed
// error (Ark error envelope when present).
func (m *ArkChatModel) doPost(ctx context.Context, body map[string]any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ark: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("ark: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ark: do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(b))
		var e struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
			msg = e.Error.Code + ": " + e.Error.Message
		}
		return nil, fmt.Errorf("ark: %s: %s", resp.Status, msg)
	}
	return resp, nil
}

var _ model.ChatModel = (*ArkChatModel)(nil)
