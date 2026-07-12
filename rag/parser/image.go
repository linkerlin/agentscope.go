package parser

import (
	"context"
	"encoding/base64"
	"io"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// ImageParser wraps an image upload as a DataBlock Section so a multimodal
// embedding model can process it downstream. An optional OCR hook can also
// extract text into a separate Section. Uses content sniffing — no filename
// assumption required.
type ImageParser struct {
	// OCR, if set, extracts text from image bytes. Called as
	// OCR(ctx, mediaType, data) -> (text, error). When nil the image is
	// emitted as a DataBlock only (no text Section).
	OCR func(ctx context.Context, mediaType string, data []byte) (string, error)
}

// NewImageParser returns an ImageParser with no OCR hook.
func NewImageParser() *ImageParser { return &ImageParser{} }

// SupportedMediaTypes returns the image MIME types this parser handles.
func (p *ImageParser) SupportedMediaTypes() []string {
	return []string{"image/png", "image/jpeg", "image/gif", "image/webp"}
}

// Parse reads the image and returns a DataBlock Section. When an OCR hook is
// configured and returns non-empty text, an additional text Section is
// prepended.
func (p *ImageParser) Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	mediaType := sniffImageMediaType(data)
	src := &message.Source{
		Type:      message.SourceTypeBase64,
		MediaType: mediaType,
		Data:      base64.StdEncoding.EncodeToString(data),
	}
	var sections []document.Section
	if p.OCR != nil {
		if text, err := p.OCR(ctx, mediaType, data); err == nil && text != "" {
			sections = append(sections, document.Section{
				Content:  message.NewTextBlock(text),
				Source:   source,
				Metadata: map[string]any{"media_type": mediaType},
			})
		}
	}
	sections = append(sections, document.Section{
		Content: message.NewDataBlock(message.TypeImage, src),
		Source:  source,
		Metadata: map[string]any{
			"media_type": mediaType,
			"bytes":      len(data),
		},
	})
	return sections, nil
}

// sniffImageMediaType detects common image formats from magic bytes.
func sniffImageMediaType(data []byte) string {
	switch {
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case len(data) >= 6 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8':
		return "image/gif"
	case len(data) >= 12 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P':
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
