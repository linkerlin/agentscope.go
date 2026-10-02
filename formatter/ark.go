// formatter/ark.go — Volcano Ark (Doubao) formatting: the wire types and the
// parsing that GUARANTEES no content block is dropped — doubao-seed responses
// carry `reasoning_content` (thinking) alongside `content` and `tool_calls`,
// and all three become message content blocks in order (thinking first).
package formatter

import (
	"encoding/json"
	"fmt"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// ArkFormatter formats messages for Volcano Ark (OpenAI-compatible chat
// shape; the thinking switch and reasoning_content handling are Ark-side).
type ArkFormatter struct {
	*OpenAIFormatter
}

var _ Formatter = (*ArkFormatter)(nil)

// NewArkFormatter builds an Ark formatter over the OpenAI shape.
func NewArkFormatter() *ArkFormatter {
	return &ArkFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

// FormatMessages implements Formatter.
func (f *ArkFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	return f.FormatMessagesTyped(msgs), nil
}

// FormatMessagesTyped reuses the OpenAI message shape.
func (f *ArkFormatter) FormatMessagesTyped(msgs []*message.Msg) []goopenai.ChatCompletionMessage {
	return f.OpenAIFormatter.FormatMessagesTyped(msgs)
}

// FormatTools implements Formatter.
func (f *ArkFormatter) FormatTools(specs []model.ToolSpec) (any, error) {
	return f.FormatToolsTyped(specs), nil
}

// FormatToolsTyped reuses the OpenAI tool shape.
func (f *ArkFormatter) FormatToolsTyped(specs []model.ToolSpec) []goopenai.Tool {
	return f.OpenAIFormatter.FormatToolsTyped(specs)
}

// FormatToolChoice implements Formatter.
func (f *ArkFormatter) FormatToolChoice(tc *model.ToolChoice) (any, error) {
	return f.OpenAIFormatter.FormatToolChoice(tc)
}

// ParseResponse implements Formatter (accepts *ArkChatResponse).
func (f *ArkFormatter) ParseResponse(resp any) (*message.Msg, error) {
	r, ok := resp.(*ArkChatResponse)
	if !ok {
		return nil, fmt.Errorf("ark formatter: expected *ArkChatResponse, got %T", resp)
	}
	if len(r.Choices) == 0 {
		return nil, fmt.Errorf("ark formatter: empty choices")
	}
	return f.ParseArkMessage(&r.Choices[0].Message), nil
}

// ---- Ark wire types ----

// ArkToolCall is one tool invocation in a response message.
type ArkToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ArkAssistantMessage is one response message: content, reasoning_content
// (thinking) and tool_calls all coexist — none may be dropped.
type ArkAssistantMessage struct {
	Content          string        `json:"content"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	ToolCalls        []ArkToolCall `json:"tool_calls,omitempty"`
}

// ArkChoice is one completion choice.
type ArkChoice struct {
	Index        int                 `json:"index"`
	Message      ArkAssistantMessage `json:"message"`
	FinishReason string              `json:"finish_reason,omitempty"`
}

// ArkError is the Ark error envelope.
type ArkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ArkUsage is the token usage block.
type ArkUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ArkChatResponse is one non-streaming chat response.
type ArkChatResponse struct {
	ID      string      `json:"id"`
	Choices []ArkChoice `json:"choices"`
	Usage   *ArkUsage   `json:"usage,omitempty"`
	Error   *ArkError   `json:"error,omitempty"`
}

// ArkStreamEvent is one SSE data event: delta carries the increments.
type ArkStreamEvent struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content          string        `json:"content,omitempty"`
			ReasoningContent string        `json:"reasoning_content,omitempty"`
			ToolCalls        []ArkToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *ArkUsage `json:"usage,omitempty"`
	Error *ArkError `json:"error,omitempty"`
}

// ParseArkMessage converts one assistant message into a Msg with EVERY
// content block preserved in order: thinking first (reasoning_content),
// then text, then tool uses.
func (f *ArkFormatter) ParseArkMessage(m *ArkAssistantMessage) *message.Msg {
	builder := message.NewMsg().Role(message.RoleAssistant)
	if m.ReasoningContent != "" {
		builder.Content(message.NewThinkingBlock(m.ReasoningContent, ""))
	}
	if m.Content != "" {
		builder.TextContent(m.Content)
	}
	for _, tc := range m.ToolCalls {
		var input map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		builder.Content(message.NewToolUseBlock(tc.ID, tc.Function.Name, input))
	}
	return builder.Build()
}

// ParseArkStreamDelta converts one SSE event into stream chunks: reasoning
// deltas are flagged IsThinking (the agent tier renders/handles thinking
// separately), content and tool deltas flow as content blocks.
func (f *ArkFormatter) ParseArkStreamDelta(ev *ArkStreamEvent) []*model.StreamChunk {
	var out []*model.StreamChunk
	for _, ch := range ev.Choices {
		if ch.Delta.ReasoningContent != "" {
			out = append(out, &model.StreamChunk{Delta: ch.Delta.ReasoningContent, IsThinking: true})
		}
		if ch.Delta.Content != "" {
			out = append(out, &model.StreamChunk{Delta: ch.Delta.Content})
		}
		for _, tc := range ch.Delta.ToolCalls {
			var input map[string]any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			out = append(out, &model.StreamChunk{
				Content: []message.ContentBlock{message.NewToolUseBlock(tc.ID, tc.Function.Name, input)},
			})
		}
	}
	return out
}

// ArkResponseFormat converts a model.ResponseFormat into the Ark
// (OpenAI-compatible) response_format object.
func ArkResponseFormat(rf *model.ResponseFormat) map[string]any {
	if rf == nil {
		return nil
	}
	switch rf.Type {
	case "json_schema":
		// model.ResponseFormat carries the schema object directly; wrap it
		// in the OpenAI-compatible json_schema envelope.
		if len(rf.JSONSchema) > 0 {
			return map[string]any{
				"type":        "json_schema",
				"json_schema": map[string]any{"schema": rf.JSONSchema},
			}
		}
		return map[string]any{"type": "json_object"}
	case "json_object", "":
		return map[string]any{"type": "json_object"}
	default:
		return map[string]any{"type": rf.Type}
	}
}
