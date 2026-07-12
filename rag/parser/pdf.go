package parser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// PDFParser extracts text from PDF files page-by-page via the pure-Go
// ledongthuc/pdf library (no CGO). Each non-empty page becomes one Section
// with metadata {"page": n}.
type PDFParser struct{}

// NewPDFParser returns a PDFParser.
func NewPDFParser() *PDFParser { return &PDFParser{} }

// SupportedMediaTypes returns the PDF MIME type.
func (p *PDFParser) SupportedMediaTypes() []string {
	return []string{"application/pdf"}
}

// Parse reads the PDF and returns one Section per non-empty page.
func (p *PDFParser) Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("pdf: %w", err)
	}
	numPages := reader.NumPage()
	sections := make([]document.Section, 0, numPages)
	for i := 1; i <= numPages; i++ {
		text := safePageText(reader.Page(i))
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		sections = append(sections, document.Section{
			Content:  message.NewTextBlock(text),
			Source:   source,
			Metadata: map[string]any{"page": i},
		})
	}
	return sections, nil
}

// safePageText extracts plain text from a page, recovering from panics that
// the underlying library may raise on malformed PDFs.
func safePageText(page pdf.Page) (text string) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
		}
	}()
	t, err := page.GetPlainText(nil)
	if err != nil {
		return ""
	}
	return t
}
