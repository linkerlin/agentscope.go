// model/moonshot/kimi.go — the native-flavoured Kimi chat line (20.4):
// Moonshot's OpenAI-compatible endpoint with the `thinking` parameter and
// `reasoning_content` responses (Kimi K2.x/K3 thinking models), implemented
// directly on net/http for the same reason as model/ark — the OpenAI SDK
// request struct cannot carry provider extensions. The wire shape (OpenAI
// messages + thinking + reasoning_content) is identical to Ark's, so the
// ArkFormatter's wire types and the no-dropped-blocks parsing are reused.
//
// MoonshotChatModelBuilder (the OpenAI SDK thin wrapper) remains for
// non-thinking use; this is the builder for the unified thinking option.
package moonshot

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

// Kimi model names (thinking-capable line).
const (
	ModelKimiK27 = "kimi-k2.7"
	ModelKimiK3  = "kimi-k3"
)

// KimiChatModel implements model.ChatModel for Moonshot's Kimi thinking
// models over net/http.
type KimiChatModel struct {
	httpClient   *http.Client
	apiKey       string
	baseURL      string
	modelName    string
	thinking     string // builder default: "" | "enabled" | "disabled"
	retryMax     int
	retryBackoff time.Duration
	fmt          *formatter.ArkFormatter
}

// KimiBuilder returns a builder for a Kimi thinking chat model.
func KimiBuilder(apiKey string) *KimiChatModelBuilder {
	return &KimiChatModelBuilder{
		apiKey:    apiKey,
		baseURL:   DefaultBaseURL,
		modelName: ModelKimiK3,
		http:      &http.Client{Timeout: 120 * time.Second},
	}
}

// KimiChatModelBuilder builds a KimiChatModel.
type KimiChatModelBuilder struct {
	apiKey    string
	baseURL   string
	modelName string
	thinking  string
	http      *http.Client
	retryMax  int
	backoff   time.Duration
}

// BaseURL overrides the API base URL (proxies, tests).
func (b *KimiChatModelBuilder) BaseURL(u string) *KimiChatModelBuilder {
	b.baseURL = u
	return b
}

// ModelName sets the Kimi model (kimi-k3, kimi-k2.7, ...).
func (b *KimiChatModelBuilder) ModelName(m string) *KimiChatModelBuilder {
	b.modelName = m
	return b
}

// Thinking sets the builder-level default (per-call WithThinking wins).
func (b *KimiChatModelBuilder) Thinking(enabled bool) *KimiChatModelBuilder {
	if enabled {
		b.thinking = "enabled"
	} else {
		b.thinking = "disabled"
	}
	return b
}

// HTTPClient sets a custom HTTP client (tests inject mocks).
func (b *KimiChatModelBuilder) HTTPClient(c *http.Client) *KimiChatModelBuilder {
	b.http = c
	return b
}

// Retry configures connection-level retry policy.
func (b *KimiChatModelBuilder) Retry(maxAttempts int, backoff time.Duration) *KimiChatModelBuilder {
	b.retryMax, b.backoff = maxAttempts, backoff
	return b
}

// Build constructs the KimiChatModel.
func (b *KimiChatModelBuilder) Build() (*KimiChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("moonshot kimi: API key is required")
	}
	httpClient := b.http
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &KimiChatModel{
		httpClient:   httpClient,
		apiKey:       b.apiKey,
		baseURL:      b.baseURL,
		modelName:    b.modelName,
		thinking:     b.thinking,
		retryMax:     b.retryMax,
		retryBackoff: b.backoff,
		fmt:          formatter.NewArkFormatter(),
	}, nil
}

// ModelName implements model.ChatModel.
func (m *KimiChatModel) ModelName() string { return m.modelName }

// Chat implements model.ChatModel.
func (m *KimiChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.retryMax >= 2 {
		var out *message.Msg
		ro := retry.Options{MaxAttempts: m.retryMax, Backoff: m.retryBackoff}
		err := retry.Do(ctx, ro, func() error {
			msg, err := m.chatOnce(ctx, messages, options...)
			if err != nil {
				return err
			}
			out = msg
			return nil
		})
		return out, err
	}
	return m.chatOnce(ctx, messages, options...)
}

func (m *KimiChatModel) chatOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
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
		return nil, fmt.Errorf("moonshot kimi: decode response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("moonshot kimi: %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("moonshot kimi: empty choices in response")
	}
	return m.fmt.ParseArkMessage(&resp.Choices[0].Message), nil
}

// ChatStream implements model.ChatModel (SSE).
func (m *KimiChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	body, err := m.buildRequest(messages, options)
	if err != nil {
		return nil, err
	}
	body["stream"] = true
	body["stream_options"] = map[string]any{"include_usage": true}
	rc, err := m.postStream(ctx, body)
	if err != nil {
		return nil, err
	}
	out := make(chan *model.StreamChunk, 16)
	go func() {
		defer close(out)
		defer rc.Close()
		scanner := bufio.NewScanner(rc)
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
				continue
			}
			if ev.Error != nil {
				out <- &model.StreamChunk{Done: true, Delta: fmt.Sprintf("moonshot kimi stream error: %s", ev.Error.Message)}
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

// buildRequest assembles the OpenAI-compatible body plus the Kimi thinking
// switch (per-call WithThinking wins over the builder default).
func (m *KimiChatModel) buildRequest(messages []*message.Msg, options []model.ChatOption) (map[string]any, error) {
	opts := model.ApplyOptions(options)
	msgsRaw, err := json.Marshal(m.fmt.FormatMessagesTyped(messages))
	if err != nil {
		return nil, fmt.Errorf("moonshot kimi: marshal messages: %w", err)
	}
	var msgs []any
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return nil, fmt.Errorf("moonshot kimi: re-message messages: %w", err)
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
	thinking := m.thinking
	if opts.Thinking != nil {
		if *opts.Thinking {
			thinking = "enabled"
		} else {
			thinking = "disabled"
		}
	}
	if thinking != "" {
		body["thinking"] = map[string]any{"type": thinking}
	}
	if len(opts.Tools) > 0 {
		toolsRaw, err := json.Marshal(m.fmt.FormatToolsTyped(opts.Tools))
		if err != nil {
			return nil, fmt.Errorf("moonshot kimi: marshal tools: %w", err)
		}
		var tools []any
		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return nil, fmt.Errorf("moonshot kimi: re-message tools: %w", err)
		}
		body["tools"] = tools
	}
	if opts.ToolChoice != nil {
		if tc, err := m.fmt.FormatToolChoice(opts.ToolChoice); err == nil && tc != nil {
			body["tool_choice"] = tc
		}
	}
	if opts.ResponseFormat != nil {
		body["response_format"] = formatter.ArkResponseFormat(opts.ResponseFormat)
	}
	return body, nil
}

func (m *KimiChatModel) postJSON(ctx context.Context, body map[string]any) ([]byte, error) {
	resp, err := m.doPost(ctx, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

func (m *KimiChatModel) postStream(ctx context.Context, body map[string]any) (io.ReadCloser, error) {
	resp, err := m.doPost(ctx, body)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (m *KimiChatModel) doPost(ctx context.Context, body map[string]any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("moonshot kimi: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("moonshot kimi: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("moonshot kimi: do request: %w", err)
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
		return nil, fmt.Errorf("moonshot kimi: %s: %s", resp.Status, msg)
	}
	return resp, nil
}

var _ model.ChatModel = (*KimiChatModel)(nil)
