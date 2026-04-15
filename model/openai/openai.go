package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	goopenai "github.com/sashabaranov/go-openai"

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
}

// OpenAIChatModelBuilder builds an OpenAIChatModel
type OpenAIChatModelBuilder struct {
	apiKey           string
	modelName        string
	baseURL          string
	retryMaxAttempts int
	retryBackoff     time.Duration
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

func (b *OpenAIChatModelBuilder) Build() (*OpenAIChatModel, error) {
	if b.apiKey == "" {
		return nil, errors.New("openai: API key is required")
	}
	cfg := goopenai.DefaultConfig(b.apiKey)
	if b.baseURL != "" {
		cfg.BaseURL = b.baseURL
	}
	return &OpenAIChatModel{
		client:           goopenai.NewClientWithConfig(cfg),
		modelName:        b.modelName,
		retryMaxAttempts: b.retryMaxAttempts,
		retryBackoff:     b.retryBackoff,
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
		Messages: msgsToOpenAI(messages),
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		req.Temperature = float32(opts.Temperature)
	}
	if len(opts.Tools) > 0 {
		req.Tools = toolSpecsToOpenAI(opts.Tools)
	}
	if opts.ToolChoice != nil {
		req.ToolChoice = toolChoiceToOpenAI(opts.ToolChoice)
	}

	resp, err := m.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai chat: no choices returned")
	}
	msg := openAIChoiceToMsg(resp.Choices[0])
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
		Messages: msgsToOpenAI(messages),
		Stream:   true,
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		req.Temperature = float32(opts.Temperature)
	}
	if len(opts.Tools) > 0 {
		req.Tools = toolSpecsToOpenAI(opts.Tools)
	}
	if opts.ToolChoice != nil {
		req.ToolChoice = toolChoiceToOpenAI(opts.ToolChoice)
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

func msgsToOpenAI(msgs []*message.Msg) []goopenai.ChatCompletionMessage {
	out := make([]goopenai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msgToOpenAI(m)...)
	}
	return out
}

func msgToOpenAI(m *message.Msg) []goopenai.ChatCompletionMessage {
	role := string(m.Role)
	toolCalls := m.GetToolUseCalls()
	toolResults := m.GetToolResults()

	if len(toolResults) > 0 {
		// One message per tool result
		out := make([]goopenai.ChatCompletionMessage, 0, len(toolResults))
		for _, tr := range toolResults {
			out = append(out, goopenai.ChatCompletionMessage{
				Role:       goopenai.ChatMessageRoleTool,
				Content:    contentBlocksToString(tr.Content),
				ToolCallID: tr.ToolUseID,
			})
		}
		return out
	}

	msg := goopenai.ChatCompletionMessage{
		Role: role,
	}
	if m.Name != "" {
		msg.Name = m.Name
	}

	if hasMediaContent(m.Content) {
		msg.MultiContent = contentBlocksToOpenAIParts(m.Content)
		msg.Content = ""
	} else {
		msg.Content = m.GetTextContent()
	}

	if len(toolCalls) > 0 {
		msg.Content = ""
		for _, tc := range toolCalls {
			inputJSON, _ := json.Marshal(tc.Input)
			msg.ToolCalls = append(msg.ToolCalls, goopenai.ToolCall{
				ID:   tc.ID,
				Type: goopenai.ToolTypeFunction,
				Function: goopenai.FunctionCall{
					Name:      tc.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}
	return []goopenai.ChatCompletionMessage{msg}
}

func hasMediaContent(blocks []message.ContentBlock) bool {
	for _, b := range blocks {
		switch b.(type) {
		case *message.ImageBlock, *message.AudioBlock, *message.VideoBlock:
			return true
		}
	}
	return false
}

func contentBlocksToOpenAIParts(blocks []message.ContentBlock) []goopenai.ChatMessagePart {
	parts := make([]goopenai.ChatMessagePart, 0, len(blocks))
	for _, b := range blocks {
		switch block := b.(type) {
		case *message.TextBlock:
			parts = append(parts, goopenai.ChatMessagePart{
				Type: goopenai.ChatMessagePartTypeText,
				Text: block.Text,
			})
		case *message.ImageBlock:
			url := block.URL
			if url == "" && block.Base64 != "" {
				mime := block.MimeType
				if mime == "" {
					mime = "image/png"
				}
				url = fmt.Sprintf("data:%s;base64,%s", mime, block.Base64)
			}
			parts = append(parts, goopenai.ChatMessagePart{
				Type: goopenai.ChatMessagePartTypeImageURL,
				ImageURL: &goopenai.ChatMessageImageURL{
					URL: url,
				},
			})
		case *message.AudioBlock:
			url := block.URL
			if url == "" && block.Base64 != "" {
				mime := block.MimeType
				if mime == "" {
					mime = "audio/wav"
				}
				url = fmt.Sprintf("data:%s;base64,%s", mime, block.Base64)
			}
			parts = append(parts, goopenai.ChatMessagePart{
				Type: goopenai.ChatMessagePartTypeText,
				Text: fmt.Sprintf("[Audio: %s]", url),
			})
		case *message.VideoBlock:
			parts = append(parts, goopenai.ChatMessagePart{
				Type: goopenai.ChatMessagePartTypeText,
				Text: fmt.Sprintf("[Video: %s]", block.URL),
			})
		case *message.ThinkingBlock:
			// skip thinking blocks when formatting for OpenAI API
		}
	}
	return parts
}

func contentBlocksToString(blocks []message.ContentBlock) string {
	var s string
	for _, b := range blocks {
		if tb, ok := b.(*message.TextBlock); ok {
			s += tb.Text
		}
	}
	return s
}

func openAIChoiceToMsg(choice goopenai.ChatCompletionChoice) *message.Msg {
	builder := message.NewMsg().Role(message.RoleAssistant)

	if choice.Message.Content != "" {
		builder.TextContent(choice.Message.Content)
	}

	for _, tc := range choice.Message.ToolCalls {
		var input map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		builder.Content(message.NewToolUseBlock(tc.ID, tc.Function.Name, input))
	}

	return builder.Build()
}

func toolSpecsToOpenAI(specs []model.ToolSpec) []goopenai.Tool {
	tools := make([]goopenai.Tool, 0, len(specs))
	for _, s := range specs {
		tools = append(tools, goopenai.Tool{
			Type: goopenai.ToolTypeFunction,
			Function: &goopenai.FunctionDefinition{
				Name:        s.Name,
				Description: s.Description,
				Parameters:  s.Parameters,
			},
		})
	}
	return tools
}

func toolChoiceToOpenAI(tc *model.ToolChoice) any {
	if tc == nil {
		return nil
	}
	if tc.Function != "" {
		return goopenai.ToolChoice{
			Type: goopenai.ToolTypeFunction,
			Function: goopenai.ToolFunction{
				Name: tc.Function,
			},
		}
	}
	return tc.Mode
}
