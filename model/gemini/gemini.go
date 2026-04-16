package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// ChatModel implements model.ChatModel for the Gemini REST API.
type ChatModel struct {
	client           *http.Client
	apiKey           string
	baseURL          string
	modelName        string
	retryMaxAttempts int
	retryBackoff     time.Duration
	fmt              *formatter.GeminiFormatter
}

// Builder constructs a ChatModel.
type Builder struct {
	apiKey           string
	baseURL          string
	modelName        string
	retryMaxAttempts int
	retryBackoff     time.Duration
	fmt              *formatter.GeminiFormatter
}

// NewBuilder returns a new Builder with defaults.
func NewBuilder() *Builder {
	return &Builder{
		modelName: "gemini-1.5-flash",
		baseURL:   defaultBaseURL,
	}
}

func (b *Builder) APIKey(key string) *Builder {
	b.apiKey = key
	return b
}

func (b *Builder) BaseURL(url string) *Builder {
	b.baseURL = url
	return b
}

func (b *Builder) ModelName(name string) *Builder {
	b.modelName = name
	return b
}

func (b *Builder) Retry(maxAttempts int, backoff time.Duration) *Builder {
	b.retryMaxAttempts = maxAttempts
	b.retryBackoff = backoff
	return b
}

func (b *Builder) Formatter(f *formatter.GeminiFormatter) *Builder {
	b.fmt = f
	return b
}

func (b *Builder) Build() (*ChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("gemini: API key is required")
	}
	f := b.fmt
	if f == nil {
		f = formatter.NewGeminiFormatter()
	}
	return &ChatModel{
		client:           &http.Client{Timeout: 120 * time.Second},
		apiKey:           b.apiKey,
		baseURL:          b.baseURL,
		modelName:        b.modelName,
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
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", m.baseURL, m.modelName, m.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini chat: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini chat: %s %s", resp.Status, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("gemini chat: %w", err)
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
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", m.baseURL, m.modelName, m.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("gemini stream: %s %s", resp.Status, string(respBody))
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
			if meta, ok := ev["usageMetadata"].(map[string]any); ok {
				usage.PromptTokens = intAny(meta["promptTokenCount"])
				usage.CompletionTokens = intAny(meta["candidatesTokenCount"])
				usage.TotalTokens = intAny(meta["totalTokenCount"])
			}
			candidates, ok := ev["candidates"].([]any)
			if !ok || len(candidates) == 0 {
				continue
			}
			candidate := candidates[0].(map[string]any)
			content, ok := candidate["content"].(map[string]any)
			if !ok {
				continue
			}
			parts, ok := content["parts"].([]any)
			if !ok {
				continue
			}
			for _, p := range parts {
				part, _ := p.(map[string]any)
				if text, ok := part["text"].(string); ok && text != "" {
					ch <- &model.StreamChunk{Delta: text}
				}
				if fc, ok := part["functionCall"].(map[string]any); ok {
					name, _ := fc["name"].(string)
					args, _ := fc["args"].(map[string]any)
					if args == nil {
						args = map[string]any{}
					}
					ch <- &model.StreamChunk{
						Content: []message.ContentBlock{message.NewToolUseBlock("", name, args)},
					}
				}
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
	contents, system := m.fmt.FormatContents(messages)

	req := map[string]any{
		"contents": contents,
	}
	if system != "" {
		req["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": system}}}
	}
	if opts.Temperature > 0 || opts.MaxTokens > 0 {
		genConfig := map[string]any{}
		if opts.Temperature > 0 {
			genConfig["temperature"] = opts.Temperature
		}
		if opts.MaxTokens > 0 {
			genConfig["maxOutputTokens"] = opts.MaxTokens
		}
		req["generationConfig"] = genConfig
	}
	if len(opts.Tools) > 0 {
		decls := m.fmt.FormatTools(opts.Tools)
		req["tools"] = []map[string]any{{"functionDeclarations": decls}}
	}
	if opts.ToolChoice != nil {
		mode := "AUTO"
		switch opts.ToolChoice.Mode {
		case "none":
			mode = "NONE"
		case "any", "required":
			mode = "ANY"
		}
		tc := map[string]any{"mode": mode}
		if opts.ToolChoice.Function != "" {
			tc["allowedFunctionNames"] = []string{opts.ToolChoice.Function}
		}
		req["toolConfig"] = map[string]any{"functionCallingConfig": tc}
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
