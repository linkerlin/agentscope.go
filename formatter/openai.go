package formatter

import (
	"encoding/json"
	"fmt"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// OpenAIFormatter formats agent messages for the OpenAI Chat Completions API.
type OpenAIFormatter struct{}

// NewOpenAIFormatter creates a new OpenAIFormatter.
func NewOpenAIFormatter() *OpenAIFormatter {
	return &OpenAIFormatter{}
}

// FormatMessages converts agent Msgs to OpenAI ChatCompletionMessages.
func (f *OpenAIFormatter) FormatMessages(msgs []*message.Msg) []goopenai.ChatCompletionMessage {
	out := make([]goopenai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, f.formatMsg(m)...)
	}
	return out
}

func (f *OpenAIFormatter) formatMsg(m *message.Msg) []goopenai.ChatCompletionMessage {
	role := string(m.Role)
	toolCalls := m.GetToolUseCalls()
	toolResults := m.GetToolResults()

	if len(toolResults) > 0 {
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
		msg.MultiContent = f.contentBlocksToParts(m.Content)
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

func (f *OpenAIFormatter) contentBlocksToParts(blocks []message.ContentBlock) []goopenai.ChatMessagePart {
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

// FormatTools converts tool specs to OpenAI Tool definitions.
func (f *OpenAIFormatter) FormatTools(specs []model.ToolSpec) []goopenai.Tool {
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

// FormatToolChoice converts a model ToolChoice to an OpenAI-compatible value.
func (f *OpenAIFormatter) FormatToolChoice(tc *model.ToolChoice) any {
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

// ParseChoice converts an OpenAI ChatCompletionChoice into a standard *message.Msg.
func (f *OpenAIFormatter) ParseChoice(choice goopenai.ChatCompletionChoice) *message.Msg {
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

// ParseResponse implements the generic Formatter interface.
func (f *OpenAIFormatter) ParseResponse(resp any) (*message.Msg, error) {
	choice, ok := resp.(goopenai.ChatCompletionChoice)
	if !ok {
		return nil, fmt.Errorf("openai formatter: expected ChatCompletionChoice, got %T", resp)
	}
	return f.ParseChoice(choice), nil
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
