package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// ExcelParser extracts tabular data from Excel (.xlsx) files.
// A .xlsx is an OOXML ZIP archive: shared strings live in
// xl/sharedStrings.xml and each sheet is in xl/worksheets/sheetN.xml.
// Cells of type t="s" reference shared strings by index.
// This parser uses only the standard library.
//
// Aligned with Python agentscope's ExcelParser (#e67e54f5).
type ExcelParser struct{}

func NewExcelParser() *ExcelParser { return &ExcelParser{} }

func (p *ExcelParser) SupportedMediaTypes() []string {
	return []string{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
}

// Parse unzips the .xlsx and returns one Section per sheet.
func (p *ExcelParser) Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("excel: not a valid zip archive: %w", err)
	}

	sharedStrings, _ := readSharedStrings(zr)
	sheets, _ := readSheetNames(zr)

	var sections []document.Section
	for _, sh := range sheets {
		rc, err := zr.Open(sh.file)
		if err != nil {
			continue
		}
		rows := parseSheetRows(rc, sharedStrings)
		rc.Close()

		if len(rows) == 0 {
			continue
		}
		md := renderMarkdownTable(rows)
		if md == "" {
			continue
		}
		sections = append(sections, document.Section{
			Content:  message.NewTextBlock(md),
			Source:   source,
			Metadata: map[string]any{"sheet": sh.name},
		})
	}
	return sections, nil
}

type sheetRef struct {
	name string
	file string
}

// readSheetNames parses xl/workbook.xml for sheet name → file mapping.
func readSheetNames(zr *zip.Reader) ([]sheetRef, error) {
	rc, err := zr.Open("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var sheets []sheetRef
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "sheet" {
			continue
		}
		var name string
		var rID string
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "name":
				name = attr.Value
			case "id":
				rID = attr.Value
			}
		}
		if name == "" {
			continue
		}
		// rId maps to sheet file via xl/_rels/workbook.xml.rels, but
		// we can just use sheet order: sheetN.xml where N = position.
		n, _ := strconv.Atoi(strings.TrimSpace(rID))
		if n > 0 {
			sheets = append(sheets, sheetRef{
				name: name,
				file: fmt.Sprintf("xl/worksheets/sheet%d.xml", n),
			})
		}
	}
	// Fallback: if workbook.xml didn't give us rIDs, scan the zip.
	if len(sheets) == 0 {
		type sf struct {
			num  int
			name string
		}
		var found []sf
		for _, f := range zr.File {
			var num int
			if _, err := fmt.Sscanf(f.Name, "xl/worksheets/sheet%d.xml", &num); err == nil && num > 0 {
				found = append(found, sf{num, f.Name})
			}
		}
		sort.Slice(found, func(i, j int) bool { return found[i].num < found[j].num })
		for i, f := range found {
			sheets = append(sheets, sheetRef{
				name: fmt.Sprintf("Sheet%d", i+1),
				file: f.name,
			})
		}
	}
	return sheets, nil
}

// readSharedStrings parses xl/sharedStrings.xml into a string slice.
func readSharedStrings(zr *zip.Reader) ([]string, error) {
	rc, err := zr.Open("xl/sharedStrings.xml")
	if err != nil {
		return nil, nil // shared strings may not exist
	}
	defer rc.Close()

	var strs []string
	dec := xml.NewDecoder(rc)
	inSI := false
	inT := false
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				inSI = true
				buf.Reset()
			}
			if t.Name.Local == "t" {
				inT = true
			}
		case xml.CharData:
			if inSI && inT {
				buf.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			}
			if t.Name.Local == "si" {
				inSI = false
				strs = append(strs, buf.String())
			}
		}
	}
	return strs, nil
}

// parseSheetRows reads worksheet XML and returns rows of cell values.
func parseSheetRows(r io.Reader, sharedStrings []string) [][]string {
	dec := xml.NewDecoder(r)
	var rows [][]string
	var currentRow []string

	inRow := false
	inCell := false
	inVal := false
	cellType := ""
	maxCol := 0
	var valBuf strings.Builder

	flushCell := func() {
		val := valBuf.String()
		if cellType == "s" {
			idx, err := strconv.Atoi(val)
			if err == nil && idx >= 0 && idx < len(sharedStrings) {
				val = sharedStrings[idx]
			}
		}
		currentRow = append(currentRow, val)
		if len(currentRow) > maxCol {
			maxCol = len(currentRow)
		}
		valBuf.Reset()
		cellType = ""
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				inRow = true
				currentRow = nil
			case "c":
				inCell = true
				valBuf.Reset()
				for _, attr := range t.Attr {
					if attr.Name.Local == "t" {
						cellType = attr.Value
					}
				}
			case "v":
				inVal = true
			case "is": // inline string
				inVal = true
			}
		case xml.CharData:
			if inVal && inCell {
				valBuf.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v", "is":
				inVal = false
			case "c":
				if inCell {
					flushCell()
					inCell = false
				}
			case "row":
				if inRow && len(currentRow) > 0 {
					rows = append(rows, currentRow)
				}
				inRow = false
			}
		}
	}

	// Pad rows to equal width for clean table rendering.
	for i := range rows {
		for len(rows[i]) < maxCol {
			rows[i] = append(rows[i], "")
		}
	}
	return rows
}
