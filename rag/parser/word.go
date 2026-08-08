package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// WordParser extracts text and tables from Word (.docx) files.
// A .docx is an OOXML ZIP archive; the main body is in word/document.xml
// with paragraphs (<w:p>), runs (<w:r>), text (<w:t>), and tables (<w:tbl>).
// This parser uses only the standard library — no external dependency.
//
// Aligned with Python agentscope's WordParser (#811425c0).
type WordParser struct{}

func NewWordParser() *WordParser { return &WordParser{} }

func (p *WordParser) SupportedMediaTypes() []string {
	return []string{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}
}

// Parse unzips the .docx and returns Sections for paragraphs and tables.
func (p *WordParser) Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("word: not a valid zip archive: %w", err)
	}

	docFile, err := zr.Open("word/document.xml")
	if err != nil {
		return nil, fmt.Errorf("word: word/document.xml not found: %w", err)
	}
	defer docFile.Close()

	return parseWordBody(docFile, source), nil
}

// parseWordBody walks the document XML, collecting paragraphs and tables.
func parseWordBody(r io.Reader, source string) []document.Section {
	dec := xml.NewDecoder(r)
	var sections []document.Section

	// State machine for walking the XML tree.
	depth := 0
	inTbl := 0      // inside <w:tbl>
	inRow := 0      // inside <w:tr>
	inCell := 0     // inside <w:tc>
	inPara := 0     // inside <w:p>
	inText := false // inside <w:t>

	var paraBuf strings.Builder // current paragraph text
	var cellBuf strings.Builder // current cell text
	var rowCells []string       // current row's cells
	var tableRows [][]string    // all rows in current table

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch t.Name.Local {
			case "tbl":
				inTbl++
				tableRows = nil
			case "tr":
				if inTbl > 0 {
					inRow++
					rowCells = nil
				}
			case "tc":
				if inRow > 0 {
					inCell++
					cellBuf.Reset()
				}
			case "p":
				inPara++
				paraBuf.Reset()
			case "t":
				inText = true
			case "br", "cr":
				if inText && inPara > 0 {
					paraBuf.WriteString("\n")
				}
			case "tab":
				if inText && inPara > 0 {
					paraBuf.WriteString("\t")
				}
			}
		case xml.CharData:
			if inText {
				if inCell > 0 {
					cellBuf.Write(t)
				} else if inPara > 0 {
					paraBuf.Write(t)
				}
			}
		case xml.EndElement:
			depth--
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				if inCell > 0 {
					// Paragraph inside a table cell: append to cell with newline
					if cellBuf.Len() > 0 {
						cellBuf.WriteString("\n")
					}
					cellBuf.WriteString(paraBuf.String())
				} else if inPara > 0 && inTbl == 0 {
					// Top-level paragraph: emit as section
					text := strings.TrimSpace(paraBuf.String())
					if text != "" {
						sections = append(sections, document.Section{
							Content:  message.NewTextBlock(text),
							Source:   source,
							Metadata: map[string]any{"type": "paragraph"},
						})
					}
				}
				if inPara > 0 {
					inPara--
				}
			case "tc":
				if inCell > 0 {
					inCell--
					rowCells = append(rowCells, strings.TrimSpace(cellBuf.String()))
				}
			case "tr":
				if inRow > 0 {
					inRow--
					if len(rowCells) > 0 {
						tableRows = append(tableRows, rowCells)
					}
				}
			case "tbl":
				if inTbl > 0 {
					inTbl--
					if len(tableRows) > 0 {
						md := renderMarkdownTable(tableRows)
						if md != "" {
							sections = append(sections, document.Section{
								Content:  message.NewTextBlock(md),
								Source:   source,
								Metadata: map[string]any{"type": "table"},
							})
						}
					}
				}
			}
		}
	}
	return sections
}

// renderMarkdownTable converts rows of cells to a Markdown pipe table.
func renderMarkdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var sb strings.Builder
	// Header
	sb.WriteString("|")
	for _, c := range rows[0] {
		sb.WriteString(" " + escapePipe(c) + " |")
	}
	sb.WriteString("\n|")
	for range rows[0] {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	// Body
	for _, row := range rows[1:] {
		sb.WriteString("|")
		for _, c := range row {
			sb.WriteString(" " + escapePipe(c) + " |")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
