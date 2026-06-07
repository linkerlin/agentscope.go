// Package openai_response provides a ChatModel implementation for the
// OpenAI Responses API. This is distinct from the Chat Completions API
// and provides first-class streaming events for reasoning, text output,
// and function-call arguments — making it a natural fit for models that
// expose chain-of-thought reasoning (e.g. o3, o4-mini).
package openai_response

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

const defaultBaseURL = "https://api.openai.com/v1"

// OpenAIResponseModel implements model.ChatModel using the OpenAI Responses API.
type OpenAIResponseModel struct {
	apiKey           string
	modelName        string
	baseURL          string
	retryMaxAttempts int
	retryBackoff     time.Duration
	formatter        *formatter.OpenAIFormatter
	thinkingEnable   bool
	reasoningEffort  string
	httpClient       *http.Client
}

// OpenAIResponseModelBuilder builds an OpenAIResponseModel.
type OpenAIResponseModelBuilder struct {
	apiKey           string
	modelName        string
	baseURL          string
	retryMaxAttempts int
	retryBackoff     time.Duration
	fmt              *formatter.OpenAIFormatter
	thinkingEnable   bool
	reasoningEffort  string
}

// Builder returns a new OpenAIResponseModelBuilder.
func Builder() *OpenAIResponseModelBuilder {
	return &OpenAIResponseModelBuilder{
		modelName: "o3",
	}
}

func (b *OpenAIResponseModelBuilder) APIKey(key string) *OpenAIResponseModelBuilder {
	b.apiKey = key
	return b
}

func (b *OpenAIResponseModelBuilder) ModelName(name string) *OpenAIResponseModelBuilder {
	b.modelName = name
	return b
}

func (b *OpenAIResponseModelBuilder) BaseURL(url string) *OpenAIResponseModelBuilder {
	b.baseURL = url
	return b
}

func (b *OpenAIResponseModelBuilder) Retry(maxAttempts int, backoff time.Duration) *OpenAIResponseModelBuilder {
	b.retryMaxAttempts = maxAttempts
	b.retryBackoff = backoff
	return b
}

func (b *OpenAIResponseModelBuilder) Formatter(f *formatter.OpenAIFormatter) *OpenAIResponseModelBuilder {
	b.fmt = f
	return b
}

func (b *OpenAIResponseModelBuilder) ThinkingEnable(enable bool) *OpenAIResponseModelBuilder {
	b.thinkingEnable = enable
	return b
}

func (b *OpenAIResponseModelBuilder) ReasoningEffort(effort string) *OpenAIResponseModelBuilder {
	b.reasoningEffort = effort
	return b
}

func (b *OpenAIResponseModelBuilder) Build() (*OpenAIResponseModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("openai_response: API key is required")
	}
	baseURL := b.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	f := b.fmt
	if f == nil {
		f = formatter.NewOpenAIFormatter()
	}
	return &OpenAIResponseModel{
		apiKey:           b.apiKey,
		modelName:        b.modelName,
		baseURL:          baseURL,
		retryMaxAttempts: b.retryMaxAttempts,
		retryBackoff:     b.retryBackoff,
		formatter:        f,
		thinkingEnable:   b.thinkingEnable,
		reasoningEffort:  b.reasoningEffort,
		httpClient:       &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (m *OpenAIResponseModel) ModelName() string { return m.modelName }

// Chat calls the Responses API (non-streaming).
func (m *OpenAIResponseModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
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

func (m *OpenAIResponseModel) chatOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	body, err := m.buildRequestBody(messages, false, options...)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai_response chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai_response chat: %s: %s", resp.Status, string(b))
	}

	var result responseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai_response chat decode: %w", err)
	}

	msg := m.parseResponseBody(&result)
	return msg, nil
}

// ChatStream calls the Responses API in streaming mode.
func (m *OpenAIResponseModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
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

func (m *OpenAIResponseModel) chatStreamOnce(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	body, err := m.buildRequestBody(messages, true, options...)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai_response stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai_response stream: %s: %s", resp.Status, string(b))
	}

	ch := make(chan *model.StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		m.parseStream(resp.Body, ch)
	}()
	return ch, nil
}

// --- request/response types ---

type requestBody struct {
	Model           string        `json:"model"`
	Input           []inputItem   `json:"input"`
	Tools           []toolItem    `json:"tools,omitempty"`
	ToolChoice      string        `json:"tool_choice,omitempty"`
	MaxOutputTokens int           `json:"max_output_tokens,omitempty"`
	Temperature     float32       `json:"temperature,omitempty"`
	Stream          bool          `json:"stream,omitempty"`
	Thinking        *thinkingOpts `json:"thinking,omitempty"`
}

type inputItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type toolItem struct {
	Type     string         `json:"type"`
	Function functionSchema `json:"function,omitempty"`
}

type functionSchema struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type thinkingOpts struct {
	Enabled bool   `json:"enabled"`
	Effort  string `json:"effort,omitempty"`
}

type responseBody struct {
	ID     string        `json:"id"`
	Output []outputItem  `json:"output"`
	Usage  *responseUsage `json:"usage,omitempty"`
}

type outputItem struct {
	Type      string        `json:"type"`
	ID        string        `json:"id,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Content   []contentItem `json:"content,omitempty"`
	Summary   string        `json:"summary,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type responseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (m *OpenAIResponseModel) buildRequestBody(messages []*message.Msg, stream bool, options ...model.ChatOption) ([]byte, error) {
	opts := applyOptions(options)

	var inputs []inputItem
	for _, msg := range messages {
		role := string(msg.Role)
		if role == "" {
			role = "user"
		}
		text := msg.GetTextContent()
		inputs = append(inputs, inputItem{Role: role, Content: text})
	}

	body := requestBody{
		Model:       m.modelName,
		Input:       inputs,
		Stream:      stream,
		Temperature: float32(opts.Temperature),
	}
	if opts.MaxTokens > 0 {
		body.MaxOutputTokens = opts.MaxTokens
	}
	if len(opts.Tools) > 0 {
		for _, t := range opts.Tools {
			body.Tools = append(body.Tools, toolItem{
				Type: "function",
				Function: functionSchema{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}
	if opts.ToolChoice != nil {
		body.ToolChoice = opts.ToolChoice.Mode
	}
	if m.thinkingEnable {
		body.Thinking = &thinkingOpts{Enabled: true, Effort: m.reasoningEffort}
	}

	return json.Marshal(body)
}

func (m *OpenAIResponseModel) parseResponseBody(result *responseBody) *message.Msg {
	msg := message.NewMsg().Role(message.RoleAssistant).Build()
	var texts []string
	var toolUses []*message.ToolUseBlock

	for _, item := range result.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					texts = append(texts, c.Text)
				}
			}
		case "function_call":
			toolUses = append(toolUses, &message.ToolUseBlock{
				ID:   item.ID,
				Name: item.Name,
				Input: func() map[string]any {
					var out map[string]any
					_ = json.Unmarshal([]byte(item.Arguments), &out)
					return out
				}(),
			})
		case "reasoning":
			if item.Summary != "" {
				texts = append(texts, "<think>"+item.Summary+"</think>")
			}
		}
	}

	if len(texts) > 0 {
		msg.Content = append(msg.Content, message.NewTextBlock(strings.Join(texts, "\n")))
	}
	for _, tu := range toolUses {
		msg.Content = append(msg.Content, tu)
	}
	if result.Usage != nil && result.Usage.TotalTokens > 0 {
		msg.Metadata["usage"] = model.ChatUsage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.TotalTokens,
		}
	}
	return msg
}

func (m *OpenAIResponseModel) parseStream(r io.Reader, ch chan<- *model.StreamChunk) {
	scanner := bufio.NewScanner(r)
	var accText strings.Builder
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

		var event streamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "response.output_text.delta":
			ch <- &model.StreamChunk{Delta: event.Delta}
			accText.WriteString(event.Delta)
		case "response.function_call_arguments.delta":
			ch <- &model.StreamChunk{Delta: event.Delta}
		case "response.reasoning_summary_text.delta":
			ch <- &model.StreamChunk{Delta: event.Delta, IsThinking: true}
		case "response.completed":
			if event.Response != nil && event.Response.Usage != nil {
				usage = model.ChatUsage{
					PromptTokens:     event.Response.Usage.InputTokens,
					CompletionTokens: event.Response.Usage.OutputTokens,
					TotalTokens:      event.Response.Usage.TotalTokens,
				}
			}
			ch <- &model.StreamChunk{Done: true, Usage: &usage}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- &model.StreamChunk{Done: true}
		return
	}
	ch <- &model.StreamChunk{Done: true, Usage: &usage}
}

type streamEvent struct {
	Type     string       `json:"type"`
	Delta    string       `json:"delta,omitempty"`
	Response *responseBody `json:"response,omitempty"`
}

func applyOptions(options []model.ChatOption) *model.ChatOptions {
	opts := &model.ChatOptions{}
	for _, o := range options {
		o(opts)
	}
	return opts
}
