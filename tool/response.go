package tool

import (
	"encoding/json"

	"github.com/linkerlin/agentscope.go/message"
)

// Response is the structured result of a tool call, aligning with Python
// agentscope.tool.ToolResponse. It supports multimedia output via
// ContentBlocks instead of a bare any-typed value.
type Response struct {
	Content       []message.ContentBlock
	Metadata      map[string]any
	Stream        bool
	IsLast        bool
	IsInterrupted bool
	ID            string
}

// NewTextResponse creates a Response with a single text block.
// If v is already a *Response it is returned as-is.
func NewTextResponse(v any) *Response {
	if r, ok := v.(*Response); ok {
		return r
	}
	var text string
	switch vv := v.(type) {
	case string:
		text = vv
	case []byte:
		text = string(vv)
	case nil:
		text = ""
	default:
		b, _ := json.Marshal(v)
		text = string(b)
	}
	return &Response{
		Content: []message.ContentBlock{message.NewTextBlock(text)},
		IsLast:  true,
	}
}

// NewErrorResponse creates a Response representing an error.
func NewErrorResponse(err error) *Response {
	return &Response{
		Content: []message.ContentBlock{message.NewTextBlock(err.Error())},
		IsLast:  true,
	}
}

// GetTextContent concatenates all text blocks into a single string.
func (r *Response) GetTextContent() string {
	if r == nil {
		return ""
	}
	var text string
	for _, b := range r.Content {
		if tb, ok := b.(*message.TextBlock); ok {
			text += tb.Text
		}
	}
	return text
}
