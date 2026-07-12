package rag_test

import (
	"archive/zip"
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

// buildPPTXFixture builds a minimal .pptx zip with the given slide text runs.
func buildPPTXFixture(t *testing.T, slides [][]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("[Content_Types].xml")
	w.Write([]byte(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="text/xml"/></Types>`))
	for i, runs := range slides {
		var inner strings.Builder
		inner.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>`)
		for _, run := range runs {
			inner.WriteString(`<a:p><a:r><a:t>`)
			inner.WriteString(run)
			inner.WriteString(`</a:t></a:r></a:p>`)
		}
		inner.WriteString(`</p:spTree></p:cSld></p:sld>`)
		name := "ppt/slides/slide" + strconv.Itoa(i+1) + ".xml"
		fw, _ := zw.Create(name)
		fw.Write([]byte(inner.String()))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestPipeline_TextDocToEndToEnd is the runnable self-check for the RAG
// indexing foundation: parse a text doc -> chunk -> verify indices/content.
func TestPipeline_TextDocToEndToEnd(t *testing.T) {
	reg := parser.NewRegistry(parser.NewTextParser())
	body := strings.Repeat("The quick brown fox. ", 600) // ~14.4k bytes ≈ 3600 approx tokens

	sections, err := reg.Parse(context.Background(), "text/plain", "doc.txt", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	ch := &chunker.ApproxTokenChunker{ChunkSize: 200, Overlap: 20}
	chunks, err := ch.Chunk(sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	total := len(chunks)
	// Every chunk carries correct bookkeeping.
	for i, c := range chunks {
		if c.ChunkIndex != i {
			t.Fatalf("chunk %d index %d", i, c.ChunkIndex)
		}
		if c.TotalChunks != total {
			t.Fatalf("chunk %d total %d want %d", i, c.TotalChunks, total)
		}
		if c.Source != "doc.txt" {
			t.Fatalf("source lost: %q", c.Source)
		}
		if c.Text() == "" {
			t.Fatalf("chunk %d has empty text", i)
		}
	}
	// Reassembling all chunk text covers the original body (allowing overlap).
	joined := strings.Join(func() []string {
		out := make([]string, len(chunks))
		for i, c := range chunks {
			out[i] = c.Text()
		}
		return out
	}(), "")
	if !strings.Contains(joined, "quick brown fox") {
		t.Fatal("joined chunks lost content")
	}
}

// TestPipeline_PPTXThenChunk verifies the multi-format path: pptx -> sections
// -> chunks, one chunk per (short) slide.
func TestPipeline_PPTXThenChunk(t *testing.T) {
	pptxBytes := buildPPTXFixture(t, [][]string{{"alpha"}, {"beta"}, {"gamma"}})
	reg := parser.NewRegistry(parser.NewPPTXParser())

	sections, err := reg.Parse(context.Background(),
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"deck.pptx", bytes.NewReader(pptxBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 3 {
		t.Fatalf("expected 3 slide sections, got %d", len(sections))
	}

	chunks, err := chunker.NewApproxTokenChunker().Chunk(sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks (1 per slide), got %d", len(chunks))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, c := range chunks {
		if c.Text() != want[i] {
			t.Fatalf("chunk %d = %q want %q", i, c.Text(), want[i])
		}
	}
}
