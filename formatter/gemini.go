package formatter

import (
	"fmt"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// GeminiFormatter formats agent messages for the Gemini REST API.
type GeminiFormatter struct{}

// NewGeminiFormatter creates a new GeminiFormatter.
func NewGeminiFormatter() *GeminiFormatter {
	return &GeminiFormatter{}
}

// FormatContents converts agent Msgs to Gemini contents and extracts a system instruction.
func (f *GeminiFormatter) FormatContents(msgs []*message.Msg) ([]map[string]any, string) {
	var out []map[string]any
	var system string
	for _, m := range msgs {
		if m.Role == message.RoleSystem {
			system = m.GetTextContent()
			continue
		}
		role := "user"
		if m.Role == message.RoleAssistant {
			role = "model"
		}
		parts := f.formatParts(m.Content)
		if len(parts) > 0 {
			out = append(out, map[string]any{"role": role, "parts": parts})
		}
	}
	return out, system
}

func (f *GeminiFormatter) formatParts(blocks []message.ContentBlock) []map[string]any {
	var out []map[string]any
	for _, b := range blocks {
		switch block := b.(type) {
		case *message.TextBlock:
			out = append(out, map[string]any{"text": block.Text})
		case *message.ThinkingBlock:
			out = append(out, map[string]any{"text": block.Thinking})
		case *message.ImageBlock:
			out = append(out, f.imagePart(block.URL, block.Base64, block.MimeType))
		case *message.AudioBlock:
			out = append(out, map[string]any{"text": fmt.Sprintf("[Audio: %s]", block.URL)})
		case *message.VideoBlock:
			out = append(out, map[string]any{"text": fmt.Sprintf("[Video: %s]", block.URL)})
		case *message.DataBlock:
			if block.Source != nil {
				switch block.BlockType() {
				case message.TypeImage:
					out = append(out, f.imagePart(block.Source.URL, block.Source.Data, block.Source.MediaType))
				default:
					out = append(out, map[string]any{"text": fmt.Sprintf("[%s: %s]", block.BlockType(), block.Source.URL)})
				}
			}
		case *message.ToolUseBlock:
			out = append(out, map[string]any{"text": fmt.Sprintf("ToolUse: %s(%v)", block.Name, block.Input)})
		case *message.ToolResultBlock:
			var text string
			for _, c := range block.Content {
				if tb, ok := c.(*message.TextBlock); ok {
					text += tb.Text
				}
			}
			out = append(out, map[string]any{"text": text})
		}
	}
	return out
}

func (f *GeminiFormatter) imagePart(url, base64, mimeType string) map[string]any {
	if url != "" {
		return map[string]any{"text": fmt.Sprintf("[Image: %s]", url)}
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return map[string]any{
		"inline_data": map[string]any{
			"mime_type": mimeType,
			"data":      base64,
		},
	}
}

// FormatTools converts tool specs to Gemini function declarations.
func (f *GeminiFormatter) FormatTools(specs []model.ToolSpec) []map[string]any {
	decls := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		decls = append(decls, map[string]any{
			"name":        s.Name,
			"description": s.Description,
			"parameters":  s.Parameters,
		})
	}
	return decls
}

// ParseResponse converts a Gemini API response into a standard *message.Msg.
func (f *GeminiFormatter) ParseResponse(body map[string]any) (*message.Msg, error) {
	candidates, ok := body["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return nil, fmt.Errorf("gemini formatter: no candidates")
	}
	candidate := candidates[0].(map[string]any)
	content, ok := candidate["content"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("gemini formatter: missing content")
	}
	parts, ok := content["parts"].([]any)
	if !ok {
		return nil, fmt.Errorf("gemini formatter: missing parts")
	}

	builder := message.NewMsg().Role(message.RoleAssistant)
	for _, p := range parts {
		part, _ := p.(map[string]any)
		if text, ok := part["text"].(string); ok {
			builder.TextContent(text)
		}
		if fc, ok := part["function_call"].(map[string]any); ok {
			name, _ := fc["name"].(string)
			args, _ := fc["args"].(map[string]any)
			if args == nil {
				args = map[string]any{}
			}
			builder.Content(message.NewToolUseBlock("", name, args))
		}
	}

	msg := builder.Build()
	if meta, ok := body["usageMetadata"].(map[string]any); ok {
		msg.Metadata["usage"] = model.ChatUsage{
			PromptTokens:     intAny(meta["promptTokenCount"]),
			CompletionTokens: intAny(meta["candidatesTokenCount"]),
			TotalTokens:      intAny(meta["totalTokenCount"]),
		}
	}
	return msg, nil
}
