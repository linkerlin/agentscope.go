package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

// buildMinimalPPTX constructs a valid .pptx zip in memory with the given
// slides (each slide's text runs).
func buildMinimalPPTX(t *testing.T, slides [][]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	// Minimal [Content_Types].xml so it's a valid OOXML package.
	_ = zwWrite(zw, t, "[Content_Types].xml", []byte(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="text/xml"/></Types>`))
	for i, runs := range slides {
		var inner strings.Builder
		inner.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>`)
		for _, run := range runs {
			inner.WriteString(`<a:p><a:r><a:t>`)
			inner.WriteString(run)
			inner.WriteString(`</a:t></a:r></a:p>`)
		}
		inner.WriteString(`</p:spTree></p:cSld></p:sld>`)
		_ = zwWrite(zw, t, "ppt/slides/slide"+itoa(i+1)+".xml", []byte(inner.String()))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zwWrite(zw *zip.Writer, t *testing.T, name string, data []byte) error {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write(data)
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestPPTXParser_BasicExtraction(t *testing.T) {
	data := buildMinimalPPTX(t, [][]string{
		{"Hello Slide 1", "Second line"},
		{"Slide 2 content"},
	})
	p := NewPPTXParser()
	secs, err := p.Parse(context.Background(), bytes.NewReader(data), "deck.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(secs))
	}
	if secs[0].Text() != "Hello Slide 1\nSecond line" {
		t.Fatalf("slide 1 text = %q", secs[0].Text())
	}
	if secs[1].Text() != "Slide 2 content" {
		t.Fatalf("slide 2 text = %q", secs[1].Text())
	}
	if secs[0].Metadata["slide"] != 1 || secs[1].Metadata["slide"] != 2 {
		t.Fatalf("slide metadata wrong: %+v", secs)
	}
	if secs[0].Source != "deck.pptx" {
		t.Fatalf("source not carried: %q", secs[0].Source)
	}
}

func TestPPTXParser_NaturalSlideOrder(t *testing.T) {
	// slide1, slide2, ..., slide10 — must come back in numeric order.
	data := buildMinimalPPTX(t, [][]string{
		{"s1"}, {"s2"}, {"s3"}, {"s4"}, {"s5"},
		{"s6"}, {"s7"}, {"s8"}, {"s9"}, {"s10"},
	})
	p := NewPPTXParser()
	secs, err := p.Parse(context.Background(), bytes.NewReader(data), "d.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 10 {
		t.Fatalf("expected 10 sections, got %d", len(secs))
	}
	if secs[9].Text() != "s10" {
		t.Fatalf("last should be s10, got %q", secs[9].Text())
	}
	if secs[9].Metadata["slide"] != 10 {
		t.Fatalf("last slide num = %v", secs[9].Metadata["slide"])
	}
}

func TestPPTXParser_InvalidZip(t *testing.T) {
	p := NewPPTXParser()
	_, err := p.Parse(context.Background(), bytes.NewReader([]byte("not a zip")), "x.pptx")
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestPPTXParser_SupportedMediaTypes(t *testing.T) {
	p := NewPPTXParser()
	if got := p.SupportedMediaTypes(); len(got) != 1 || !strings.Contains(got[0], "presentationml") {
		t.Fatalf("unexpected media types: %v", got)
	}
}

func TestPDFParser_InvalidInput(t *testing.T) {
	p := NewPDFParser()
	_, err := p.Parse(context.Background(), bytes.NewReader([]byte("not a pdf")), "x.pdf")
	if err == nil {
		t.Fatal("expected error for invalid PDF")
	}
}

func TestPDFParser_SupportedMediaTypes(t *testing.T) {
	if got := NewPDFParser().SupportedMediaTypes(); len(got) != 1 || got[0] != "application/pdf" {
		t.Fatalf("unexpected: %v", got)
	}
}
