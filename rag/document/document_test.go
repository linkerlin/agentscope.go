package document

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestSection_Text(t *testing.T) {
	s := Section{Content: message.NewTextBlock("hello"), Source: "a.txt"}
	if s.Text() != "hello" {
		t.Fatalf("got %q", s.Text())
	}
	if s.IsData() {
		t.Fatal("text section should not be data")
	}
}

func TestSection_DataTextEmpty(t *testing.T) {
	s := Section{
		Content: message.NewDataBlock(message.TypeImage, &message.Source{URL: "x.png"}),
		Source:  "a.png",
	}
	if s.Text() != "" {
		t.Fatalf("data section text should be empty, got %q", s.Text())
	}
	if !s.IsData() {
		t.Fatal("expected data section")
	}
}

func TestChunk_Text(t *testing.T) {
	c := Chunk{Content: message.NewTextBlock("body"), Source: "a.txt", ChunkIndex: 2, TotalChunks: 5}
	if c.Text() != "body" {
		t.Fatalf("got %q", c.Text())
	}
	if c.ChunkIndex != 2 || c.TotalChunks != 5 {
		t.Fatal("index fields not preserved")
	}
}
