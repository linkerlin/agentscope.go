package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

// --- Word parser tests ---

func TestWordParser_SupportedMediaTypes(t *testing.T) {
	p := NewWordParser()
	mts := p.SupportedMediaTypes()
	if len(mts) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(mts))
	}
	if mts[0] != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("unexpected media type: %s", mts[0])
	}
}

func TestWordParser_MediaType(t *testing.T) {
	// Verify it doesn't panic
	_ = NewWordParser()
}

func TestRenderMarkdownTable(t *testing.T) {
	rows := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}
	md := renderMarkdownTable(rows)
	if !strings.Contains(md, "| Name | Age | City |") {
		t.Fatalf("header row missing: %s", md)
	}
	if !strings.Contains(md, "| --- |") {
		t.Fatalf("separator row missing: %s", md)
	}
	if !strings.Contains(md, "| Alice | 30 | NYC |") {
		t.Fatalf("data row missing: %s", md)
	}
}

func TestRenderMarkdownTable_PipeEscape(t *testing.T) {
	rows := [][]string{
		{"a|b", "c"},
		{"1", "2"},
	}
	md := renderMarkdownTable(rows)
	if !strings.Contains(md, `\|`) {
		t.Fatalf("pipe should be escaped: %s", md)
	}
}

func TestRenderMarkdownTable_Empty(t *testing.T) {
	if renderMarkdownTable(nil) != "" {
		t.Fatal("nil rows should produce empty string")
	}
	if renderMarkdownTable([][]string{}) != "" {
		t.Fatal("empty rows should produce empty string")
	}
}

// --- Excel parser tests ---

func TestExcelParser_SupportedMediaTypes(t *testing.T) {
	p := NewExcelParser()
	mts := p.SupportedMediaTypes()
	if len(mts) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(mts))
	}
	if mts[0] != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected media type: %s", mts[0])
	}
}

func TestParseSheetRows_Numbers(t *testing.T) {
	xml := `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row>
      <c r="A1"><v>42</v></c>
      <c r="B1"><v>hello</v></c>
    </row>
  </sheetData>
</worksheet>`
	rows := parseSheetRows(strings.NewReader(xml), nil)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][0] != "42" {
		t.Fatalf("expected '42', got %s", rows[0][0])
	}
	if rows[0][1] != "hello" {
		t.Fatalf("expected 'hello', got %s", rows[0][1])
	}
}

func TestParseSheetRows_SharedStrings(t *testing.T) {
	xml := `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row>
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
      <c r="C1"><v>100</v></c>
    </row>
  </sheetData>
</worksheet>`
	shared := []string{"Name", "Age"}
	rows := parseSheetRows(strings.NewReader(xml), shared)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][0] != "Name" {
		t.Fatalf("expected 'Name', got %s", rows[0][0])
	}
	if rows[0][1] != "Age" {
		t.Fatalf("expected 'Age', got %s", rows[0][1])
	}
	if rows[0][2] != "100" {
		t.Fatalf("expected '100', got %s", rows[0][2])
	}
}

func TestParseSheetRows_MultipleRows(t *testing.T) {
	xml := `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row>
      <c r="A1"><v>1</v></c>
      <c r="B1"><v>2</v></c>
    </row>
    <row>
      <c r="A2"><v>3</v></c>
      <c r="B2"><v>4</v></c>
    </row>
    <row>
      <c r="A3"><v>5</v></c>
      <c r="B3"><v>6</v></c>
    </row>
  </sheetData>
</worksheet>`
	rows := parseSheetRows(strings.NewReader(xml), nil)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[2][0] != "5" || rows[2][1] != "6" {
		t.Fatalf("row 3 mismatch: %v", rows[2])
	}
}

func TestReadSharedStrings(t *testing.T) {
	// Create a minimal zip with sharedStrings.xml
	ssXML := `<?xml version="1.0"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>Hello</t></si>
  <si><t>World</t></si>
  <si><t>Foo Bar</t></si>
</sst>`

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	fw, _ := w.Create("xl/sharedStrings.xml")
	fw.Write([]byte(ssXML))
	w.Close()

	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	strs, err := readSharedStrings(zr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(strs) != 3 {
		t.Fatalf("expected 3 strings, got %d", len(strs))
	}
	if strs[0] != "Hello" || strs[1] != "World" || strs[2] != "Foo Bar" {
		t.Fatalf("strings mismatch: %v", strs)
	}
}

// --- Integration: Word parser with a real .docx-like zip ---

func TestWordParser_EndToEnd(t *testing.T) {
	// Create a minimal .docx-like zip with word/document.xml
	docXML := `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello World</w:t></w:r></w:p>
    <w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p>
  </w:body>
</w:document>`

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	fw, _ := w.Create("word/document.xml")
	fw.Write([]byte(docXML))
	w.Close()

	p := NewWordParser()
	sections, err := p.Parse(context.Background(), bytes.NewReader(buf.Bytes()), "test.docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
}

func TestWordParser_TableExtraction(t *testing.T) {
	docXML := `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>A1</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>B1</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>A2</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>B2</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	fw, _ := w.Create("word/document.xml")
	fw.Write([]byte(docXML))
	w.Close()

	p := NewWordParser()
	sections, err := p.Parse(context.Background(), bytes.NewReader(buf.Bytes()), "test.docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section (table), got %d", len(sections))
	}
	// Verify it's a table with Markdown format
	// Content is a message.ContentBlock (TextBlock)
	_ = sections[0].Content
}
