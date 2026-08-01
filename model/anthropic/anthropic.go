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
	"sort"
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
		toolUses := map[int]*toolUseAccum{}
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				if usage.TotalTokens == 0 {
					usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
				}
				ch <- &model.StreamChunk{Done: true, Usage: &usage, Content: finishToolBlocks(toolUses)}
				return
			}
			var ev map[string]any
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			if ev["type"] == "message_start" {
				if u, ok := ev["usage"].(map[string]any); ok {
					usage.PromptTokens = intAny(u["input_tokens"])
					if usage.CompletionTokens == 0 {
						usage.CompletionTokens = intAny(u["output_tokens"])
					}
				}
			}
			if ev["type"] == "message_delta" {
				if u, ok := ev["usage"].(map[string]any); ok {
					usage.CompletionTokens = intAny(u["output_tokens"])
					if usage.PromptTokens == 0 {
						usage.PromptTokens = intAny(u["input_tokens"])
					}
				}
			}
			switch ev["type"] {
			case "content_block_start":
				if cb, _ := ev["content_block"].(map[string]any); cb != nil && cb["type"] == "tool_use" {
					idx := intAny(ev["index"])
					id, _ := cb["id"].(string)
					name, _ := cb["name"].(string)
					toolUses[idx] = &toolUseAccum{id: id, name: name}
				}
			case "content_block_delta":
				delta, ok := ev["delta"].(map[string]any)
				if !ok {
					continue
				}
				if dj, _ := delta["type"].(string); dj == "input_json_delta" {
					if acc := toolUses[intAny(ev["index"])]; acc != nil {
						if pj, _ := delta["partial_json"].(string); pj != "" {
							acc.args.WriteString(pj)
						}
					}
					continue
				}
				text, _ := delta["text"].(string)
				if text != "" {
					ch <- &model.StreamChunk{Delta: text}
				}
				if thinking, ok := delta["thinking"].(string); ok && thinking != "" {
					ch <- &model.StreamChunk{Content: []message.ContentBlock{message.NewThinkingBlock(thinking, "")}}
				}
			case "message_stop":
				// Anthropic 标准终止信号;[DONE] 分支保留作代理/网关兼容。
				if usage.TotalTokens == 0 {
					usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
				}
				ch <- &model.StreamChunk{Done: true, Usage: &usage, Content: finishToolBlocks(toolUses)}
				return
			case "error":
				// 流中途错误(超限/内容审核等),透传而不是静默空响应。
				msg := "anthropic stream error"
				if e, ok := ev["error"].(map[string]any); ok {
					if m, _ := e["message"].(string); m != "" {
						msg = m
					}
				}
				ch <- &model.StreamChunk{Done: true, Error: fmt.Errorf("anthropic stream: %s", msg)}
				return
			}
		}
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		ch <- &model.StreamChunk{Done: true, Usage: &usage, Content: finishToolBlocks(toolUses)}
	}()
	return ch, nil
}

func (m *ChatModel) buildRequestBody(messages []*message.Msg, stream bool, options ...model.ChatOption) ([]byte, error) {
	opts := &model.ChatOptions{}
	for _, o := range options {
		o(opts)
	}
	amsgs, system := m.extractMessages(messages)

	maxTokens := m.maxTokens
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}
	req := map[string]any{
		"model":      m.modelName,
		"max_tokens": maxTokens,
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
		tools, _ := m.fmt.FormatTools(opts.Tools)
		req["tools"] = tools
	}
	if opts.ToolChoice != nil {
		tc, _ := m.fmt.FormatToolChoice(opts.ToolChoice)
		req["tool_choice"] = tc
	}
	return json.Marshal(req)
}

// extractMessages calls FormatMessages via the Formatter interface and extracts the typed result.
func (m *ChatModel) extractMessages(messages []*message.Msg) (any, string) {
	raw, err := m.fmt.FormatMessages(messages)
	if err != nil {
		return nil, ""
	}
	result, ok := raw.(formatter.AnthropicFormatResult)
	if !ok {
		return nil, ""
	}
	return result.Messages, result.SystemPrompt
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

// toolUseAccum aggregates streaming tool_use content for a single content_block index.
type toolUseAccum struct {
	id   string
	name string
	args strings.Builder
}

func (a *toolUseAccum) toBlock() *message.ToolUseBlock {
	if a == nil || a.name == "" {
		return nil
	}
	args := a.args.String()
	var input map[string]any
	if args != "" {
		_ = json.Unmarshal([]byte(args), &input)
	}
	b := message.NewToolUseBlock(a.id, a.name, input)
	b.RawInput = args
	return b
}

// finishToolBlocks assembles accumulated tool_use blocks in index order.
func finishToolBlocks(m map[int]*toolUseAccum) []message.ContentBlock {
	if len(m) == 0 {
		return nil
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var blocks []message.ContentBlock
	for _, k := range keys {
		if b := m[k].toBlock(); b != nil {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

var _ model.ChatModel = (*ChatModel)(nil)
