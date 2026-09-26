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
	"sort"
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

// NewBuilder is an alias for Builder, following the Go New-prefix convention.
func NewBuilder() *OpenAIResponseModelBuilder { return Builder() }

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
		m.parseStream(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// --- request/response types ---

type requestBody struct {
	Model           string        `json:"model"`
	Input           []any         `json:"input"`
	Tools           []toolItem    `json:"tools,omitempty"`
	ToolChoice      string        `json:"tool_choice,omitempty"`
	MaxOutputTokens int           `json:"max_output_tokens,omitempty"`
	Temperature     float32       `json:"temperature,omitempty"`
	Stream          bool          `json:"stream,omitempty"`
	Thinking        *thinkingOpts `json:"thinking,omitempty"`
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
	ID     string         `json:"id"`
	Output []outputItem   `json:"output"`
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
	InputTokens       int                  `json:"input_tokens"`
	OutputTokens      int                  `json:"output_tokens"`
	TotalTokens       int                  `json:"total_tokens"`
	InputTokenDetails *responseTokenDetail `json:"input_token_details,omitempty"`
}

// responseTokenDetail carries the Responses API token breakdown, notably the
// prompt-cache hit counter.
type responseTokenDetail struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// chatUsageFromResponse converts Responses usage, preserving the cached-token
// counter (E8a).
func chatUsageFromResponse(u *responseUsage) model.ChatUsage {
	out := model.ChatUsage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}
	if u.InputTokenDetails != nil {
		out.CachedPromptTokens = u.InputTokenDetails.CachedTokens
	}
	return out
}

func (m *OpenAIResponseModel) buildRequestBody(messages []*message.Msg, stream bool, options ...model.ChatOption) ([]byte, error) {
	opts := applyOptions(options)

	// Build native Responses input items so tool calls and their results
	// round-trip across turns. Text-only flattening would drop the
	// function_call/function_call_output pairing and break multi-turn tool
	// use entirely (E5c).
	var inputs []any
	for _, msg := range messages {
		switch msg.Role {
		case message.RoleAssistant:
			var texts []string
			for _, b := range msg.Content {
				switch v := b.(type) {
				case *message.TextBlock:
					if v.Text != "" {
						texts = append(texts, v.Text)
					}
				case *message.ToolUseBlock:
					inputs = append(inputs, map[string]any{
						"type":      "function_call",
						"call_id":   v.ID,
						"name":      v.Name,
						"arguments": marshalToolInput(v.Input),
					})
				}
			}
			if len(texts) > 0 {
				inputs = append(inputs, map[string]any{
					"role":    "assistant",
					"content": strings.Join(texts, "\n"),
				})
			}
		case message.RoleTool:
			for _, b := range msg.Content {
				tr, ok := b.(*message.ToolResultBlock)
				if !ok {
					continue
				}
				inputs = append(inputs, map[string]any{
					"type":    "function_call_output",
					"call_id": tr.ToolUseID,
					"output":  toolResultOutput(tr),
				})
			}
		default:
			role := string(msg.Role)
			if role == "" {
				role = "user"
			}
			inputs = append(inputs, map[string]any{
				"role":    role,
				"content": msg.GetTextContent(),
			})
		}
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

// marshalToolInput renders a tool-call argument map as JSON for the
// function_call input item.
func marshalToolInput(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// toolResultText flattens a tool result to its text blocks for the
// function_call_output item.
func toolResultText(block *message.ToolResultBlock) string {
	var sb strings.Builder
	for _, c := range block.Content {
		if tb, ok := c.(*message.TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	return sb.String()
}

// toolResultOutput renders a tool result for the function_call_output item.
// Text-only results keep the compact string form; results carrying images or
// files use the native content-part array (input_text/input_image/input_file)
// so media survives the round-trip instead of being dropped.
func toolResultOutput(block *message.ToolResultBlock) any {
	if !hasResponseMedia(block.Content) {
		return toolResultText(block)
	}
	var parts []map[string]any
	for _, c := range block.Content {
		switch v := c.(type) {
		case *message.TextBlock:
			if v.Text != "" {
				parts = append(parts, map[string]any{"type": "input_text", "text": v.Text})
			}
		case *message.ImageBlock:
			if url := responseMediaURL(v.URL, v.Base64, v.MimeType, "image/png"); url != "" {
				parts = append(parts, map[string]any{"type": "input_image", "image_url": url})
			}
		case *message.DataBlock:
			if v.Source == nil {
				continue
			}
			switch v.BlockType() {
			case message.TypeImage:
				if url := responseMediaURL(v.Source.URL, v.Source.Data, v.Source.MediaType, "image/png"); url != "" {
					parts = append(parts, map[string]any{"type": "input_image", "image_url": url})
				}
			case message.TypeData:
				// Generic files (e.g. PDFs) travel as input_file parts.
				if url := responseMediaURL(v.Source.URL, v.Source.Data, v.Source.MediaType, "application/pdf"); url != "" {
					parts = append(parts, map[string]any{"type": "input_file", "file_data": url})
				}
			}
		}
	}
	if len(parts) == 0 {
		return toolResultText(block)
	}
	return parts
}

// hasResponseMedia reports whether tool-result content carries image or file
// blocks worth preserving as native output parts.
func hasResponseMedia(blocks []message.ContentBlock) bool {
	for _, b := range blocks {
		switch v := b.(type) {
		case *message.ImageBlock:
			return true
		case *message.DataBlock:
			if v.Source != nil && (v.BlockType() == message.TypeImage || v.BlockType() == message.TypeData) {
				return true
			}
		}
	}
	return false
}

// responseMediaURL builds a data URL from URL or base64 payload.
func responseMediaURL(url, base64, mimeType, defaultMime string) string {
	if url != "" {
		return url
	}
	if base64 == "" {
		return ""
	}
	if mimeType == "" {
		mimeType = defaultMime
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64)
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
			// The Responses API keys function_call_output by call_id, so the
			// ToolUseBlock ID must be the call_id (falling back to the item
			// id) for multi-turn tool use to round-trip (E5c).
			callID := item.CallID
			if callID == "" {
				callID = item.ID
			}
			toolUses = append(toolUses, &message.ToolUseBlock{
				ID:   callID,
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
		msg.Metadata["usage"] = chatUsageFromResponse(result.Usage)
	}
	return msg
}

func (m *OpenAIResponseModel) parseStream(ctx context.Context, r io.Reader, ch chan<- *model.StreamChunk) {
	scanner := bufio.NewScanner(r)
	var accText strings.Builder
	var usage model.ChatUsage
	// funcAccums collects function-call arguments by output index so the
	// parameter JSON is assembled into ToolUseBlocks instead of leaking into
	// the answer text (E5b).
	funcAccums := map[int]*funcCallAccum{}
	// send delivers a chunk unless the caller cancelled the request; a
	// cancelled consumer must not leak this goroutine and the HTTP body (E5d).
	send := func(c *model.StreamChunk) bool {
		select {
		case ch <- c:
			return true
		case <-ctx.Done():
			return false
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			send(&model.StreamChunk{Done: true, Usage: &usage, Content: finishFuncCalls(funcAccums)})
			return
		}

		var event streamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "response.output_text.delta":
			if !send(&model.StreamChunk{Delta: event.Delta}) {
				return
			}
			accText.WriteString(event.Delta)
		case "response.output_item.added":
			if event.Item != nil && event.Item.Type == "function_call" {
				funcAccums[event.OutputIndex] = &funcCallAccum{
					id:     event.Item.ID,
					callID: event.Item.CallID,
					name:   event.Item.Name,
				}
			}
		case "response.function_call_arguments.delta":
			acc := funcAccums[event.OutputIndex]
			if acc == nil {
				acc = &funcCallAccum{}
				funcAccums[event.OutputIndex] = acc
			}
			acc.args.WriteString(event.Delta)
		case "response.function_call_arguments.done":
			acc := funcAccums[event.OutputIndex]
			if acc == nil {
				acc = &funcCallAccum{}
				funcAccums[event.OutputIndex] = acc
			}
			if event.Arguments != "" {
				acc.args.Reset()
				acc.args.WriteString(event.Arguments)
			}
		case "response.reasoning_summary_text.delta":
			if !send(&model.StreamChunk{Delta: event.Delta, IsThinking: true}) {
				return
			}
		case "response.completed":
			if event.Response != nil && event.Response.Usage != nil {
				usage = chatUsageFromResponse(event.Response.Usage)
			}
			send(&model.StreamChunk{Done: true, Usage: &usage, Content: finishFuncCalls(funcAccums)})
			return
		}
	}

	if err := scanner.Err(); err != nil {
		send(&model.StreamChunk{Done: true})
		return
	}
	send(&model.StreamChunk{Done: true, Usage: &usage, Content: finishFuncCalls(funcAccums)})
}

// funcCallAccum assembles one streaming function call.
type funcCallAccum struct {
	id     string
	callID string
	name   string
	args   strings.Builder
}

// finishFuncCalls converts accumulated streaming function calls into
// ToolUseBlocks ordered by output index. The block ID is the Responses
// call_id so the later function_call_output round-trips (E5c).
func finishFuncCalls(accums map[int]*funcCallAccum) []message.ContentBlock {
	if len(accums) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(accums))
	for i := range accums {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)
	var out []message.ContentBlock
	for _, i := range indexes {
		acc := accums[i]
		var input map[string]any
		if args := acc.args.String(); args != "" {
			_ = json.Unmarshal([]byte(args), &input)
		}
		if input == nil {
			input = map[string]any{}
		}
		id := acc.callID
		if id == "" {
			id = acc.id
		}
		out = append(out, &message.ToolUseBlock{ID: id, Name: acc.name, Input: input})
	}
	return out
}

type streamEvent struct {
	Type        string        `json:"type"`
	Delta       string        `json:"delta,omitempty"`
	OutputIndex int           `json:"output_index,omitempty"`
	ItemID      string        `json:"item_id,omitempty"`
	Item        *outputItem   `json:"item,omitempty"`
	Arguments   string        `json:"arguments,omitempty"`
	Response    *responseBody `json:"response,omitempty"`
}

func applyOptions(options []model.ChatOption) *model.ChatOptions {
	opts := &model.ChatOptions{}
	for _, o := range options {
		o(opts)
	}
	return opts
}
