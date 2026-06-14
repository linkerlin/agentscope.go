package formatter

import (
	"encoding/json"
	"fmt"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// AnthropicFormatter formats agent messages for the Anthropic Messages API.
type AnthropicFormatter struct{}

// NewAnthropicFormatter creates a new AnthropicFormatter.
func NewAnthropicFormatter() *AnthropicFormatter {
	return &AnthropicFormatter{}
}

// anthropicMessage represents a single message in the Anthropic API.
type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// AnthropicFormatResult holds the result of formatting messages for the Anthropic API.
// Anthropic requires the system prompt to be passed separately from the message list.
type AnthropicFormatResult struct {
	Messages     []anthropicMessage `json:"messages"`
	SystemPrompt string             `json:"system_prompt,omitempty"`
}

// FormatMessages converts agent Msgs to Anthropic messages and extracts the system prompt.
// Implements Formatter interface. Use type assertion on the result to access typed fields.
func (f *AnthropicFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	var out []anthropicMessage
	var systemPrompt string
	for _, m := range msgs {
		if m.Role == message.RoleSystem {
			systemPrompt = m.GetTextContent()
			continue
		}
		content := f.formatContentBlocks(m.Content)
		if len(content) == 0 {
			continue
		}
		role := string(m.Role)
		if role == "tool" {
			role = "user"
		}
		data, _ := json.Marshal(content)
		out = append(out, anthropicMessage{Role: role, Content: data})
	}
	return AnthropicFormatResult{Messages: out, SystemPrompt: systemPrompt}, nil
}

func (f *AnthropicFormatter) formatContentBlocks(blocks []message.ContentBlock) []map[string]any {
	var out []map[string]any
	for _, b := range blocks {
		switch block := b.(type) {
		case *message.TextBlock:
			out = append(out, map[string]any{"type": "text", "text": block.Text})
		case *message.ThinkingBlock:
			out = append(out, map[string]any{"type": "thinking", "thinking": block.Thinking, "signature": block.Signature})
		case *message.ImageBlock:
			src := f.imageSource(block.URL, block.Base64, block.MimeType)
			out = append(out, map[string]any{"type": "image", "source": src})
		case *message.AudioBlock:
			// Anthropic doesn't natively support audio in the same way; treat as text hint
			out = append(out, map[string]any{"type": "text", "text": fmt.Sprintf("[Audio: %s]", block.URL)})
		case *message.VideoBlock:
			out = append(out, map[string]any{"type": "text", "text": fmt.Sprintf("[Video: %s]", block.URL)})
		case *message.DataBlock:
			if block.Source != nil {
				switch block.BlockType() {
				case message.TypeImage:
					src := f.imageSource(block.Source.URL, block.Source.Data, block.Source.MediaType)
					out = append(out, map[string]any{"type": "image", "source": src})
				default:
					out = append(out, map[string]any{"type": "text", "text": fmt.Sprintf("[%s: %s]", block.BlockType(), block.Source.URL)})
				}
			}
		case *message.ToolUseBlock:
			out = append(out, map[string]any{
				"type":  "tool_use",
				"id":    block.ID,
				"name":  block.Name,
				"input": block.Input,
			})
		case *message.ToolResultBlock:
			var result string
			for _, c := range block.Content {
				if tb, ok := c.(*message.TextBlock); ok {
					result += tb.Text
				}
			}
			out = append(out, map[string]any{
				"type":        "tool_result",
				"tool_use_id": block.ToolUseID,
				"content":     result,
			})
		}
	}
	return out
}

func (f *AnthropicFormatter) imageSource(url, base64, mimeType string) map[string]any {
	if url != "" {
		return map[string]any{"type": "url", "url": url}
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return map[string]any{"type": "base64", "media_type": mimeType, "data": base64}
}

// FormatTools converts tool specs to Anthropic tool definitions.
// Implements Formatter interface.
func (f *AnthropicFormatter) FormatTools(specs []model.ToolSpec) (any, error) {
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		out = append(out, map[string]any{
			"name":         s.Name,
			"description":  s.Description,
			"input_schema": s.Parameters,
		})
	}
	return out, nil
}

// FormatToolChoice converts a model ToolChoice to Anthropic representation.
// Implements Formatter interface.
func (f *AnthropicFormatter) FormatToolChoice(tc *model.ToolChoice) (any, error) {
	if tc == nil {
		return map[string]any{"type": "auto"}, nil
	}
	if tc.Function != "" {
		return map[string]any{"type": "tool", "name": tc.Function}, nil
	}
	switch tc.Mode {
	case "none":
		return map[string]any{"type": "none"}, nil
	case "required", "any":
		return map[string]any{"type": "any"}, nil
	default:
		return map[string]any{"type": "auto"}, nil
	}
}

// ParseResponse converts an Anthropic API response into a standard *message.Msg.
// Implements Formatter interface. The resp parameter should be map[string]any.
func (f *AnthropicFormatter) ParseResponse(resp any) (*message.Msg, error) {
	body, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("anthropic formatter: ParseResponse expects map[string]any, got %T", resp)
	}
	contentRaw, ok := body["content"].([]any)
	if !ok {
		return nil, fmt.Errorf("anthropic formatter: invalid content field")
	}

	builder := message.NewMsg().Role(message.RoleAssistant)
	for _, item := range contentRaw {
		m, _ := item.(map[string]any)
		typ, _ := m["type"].(string)
		switch typ {
		case "text":
			text, _ := m["text"].(string)
			builder.TextContent(text)
		case "thinking":
			thinking, _ := m["thinking"].(string)
			sig, _ := m["signature"].(string)
			builder.Content(message.NewThinkingBlock(thinking, sig))
		case "tool_use":
			id, _ := m["id"].(string)
			name, _ := m["name"].(string)
			input, _ := m["input"].(map[string]any)
			if input == nil {
				input = map[string]any{}
			}
			builder.Content(message.NewToolUseBlock(id, name, input))
		}
	}

	msg := builder.Build()
	if usageRaw, ok := body["usage"].(map[string]any); ok {
		msg.Metadata["usage"] = model.ChatUsage{
			PromptTokens:     intAny(usageRaw["input_tokens"]),
			CompletionTokens: intAny(usageRaw["output_tokens"]),
			TotalTokens:      intAny(usageRaw["input_tokens"]) + intAny(usageRaw["output_tokens"]),
		}
	}
	return msg, nil
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

// WrapThinkingBlock wraps raw thinking content with Anthropic's native
// <thinking> tags. This satisfies the ThinkingFormatter extension interface.
func (f *AnthropicFormatter) WrapThinkingBlock(content string) string {
	return "<thinking>\n" + content + "\n</thinking>"
}

var _ Formatter = (*AnthropicFormatter)(nil)
var _ ThinkingFormatter = (*AnthropicFormatter)(nil)
