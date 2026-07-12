package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// PPTXParser extracts text from PowerPoint (.pptx) files. A .pptx is an
// OOXML ZIP archive; slide text lives in ppt/slides/slideN.xml as <a:t>
// DrawingML runs. This parser uses only the standard library (archive/zip +
// encoding/xml) — no external dependency. Each slide becomes one Section
// with metadata {"slide": n}.
type PPTXParser struct{}

// NewPPTXParser returns a PPTXParser.
func NewPPTXParser() *PPTXParser { return &PPTXParser{} }

// SupportedMediaTypes returns the PPTX MIME type.
func (p *PPTXParser) SupportedMediaTypes() []string {
	return []string{
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	}
}

var slideFileRe = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

// Parse unzips the .pptx and returns one Section per slide with text.
func (p *PPTXParser) Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("pptx: not a valid zip archive: %w", err)
	}

	type slideFile struct {
		num  int
		name string
	}
	var slides []slideFile
	for _, f := range zr.File {
		if m := slideFileRe.FindStringSubmatch(f.Name); m != nil {
			n, _ := strconv.Atoi(m[1])
			slides = append(slides, slideFile{num: n, name: f.Name})
		}
	}
	// Natural sort by slide number (slide2 before slide10).
	sort.Slice(slides, func(i, j int) bool { return slides[i].num < slides[j].num })

	sections := make([]document.Section, 0, len(slides))
	for _, s := range slides {
		rc, err := zr.Open(s.name)
		if err != nil {
			continue
		}
		text := extractSlideText(rc)
		rc.Close()
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		sections = append(sections, document.Section{
			Content:  message.NewTextBlock(text),
			Source:   source,
			Metadata: map[string]any{"slide": s.num},
		})
	}
	return sections, nil
}

// extractSlideText collects all text from DrawingML <a:t> runs in a slide.
// Text runs are joined with newlines for readability.
func extractSlideText(r io.Reader) string {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	inT := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
			}
		case xml.CharData:
			if inT {
				sb.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				if inT {
					sb.WriteString("\n")
					inT = false
				}
			}
		}
	}
	return sb.String()
}
