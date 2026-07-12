package parser

import (
	"context"
	"io"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// TextParser handles plain-text and lightweight markup uploads. It emits a
// single Section for the whole file — text is split downstream by a Chunker.
// No external dependency.
type TextParser struct {
	// MediaTypes overrides the default supported types. If empty, defaults to
	// text/plain, text/markdown, text/csv, application/json.
	MediaTypes []string
}

// NewTextParser returns a TextParser with the default supported media types.
func NewTextParser() *TextParser { return &TextParser{} }

// SupportedMediaTypes returns the MIME types this parser handles.
func (p *TextParser) SupportedMediaTypes() []string {
	if len(p.MediaTypes) > 0 {
		return p.MediaTypes
	}
	return []string{
		"text/plain",
		"text/markdown",
		"text/csv",
		"application/json",
		"application/x-yaml",
		"text/yaml",
	}
}

// Parse reads all bytes from r and returns one text Section.
func (p *TextParser) Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return []document.Section{{
		Content: message.NewTextBlock(string(data)),
		Source:  source,
	}}, nil
}
