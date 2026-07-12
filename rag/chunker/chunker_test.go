package chunker

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

func TestSplitApproxTokens_ShortText(t *testing.T) {
	got := splitApproxTokens("hello", 512, 50)
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("expected single chunk, got %v", got)
	}
}

func TestSplitApproxTokens_Empty(t *testing.T) {
	if got := splitApproxTokens("", 512, 50); len(got) != 0 {
		t.Fatalf("expected no chunks, got %v", got)
	}
}

func TestSplitApproxTokens_MultiChunk(t *testing.T) {
	// 5000 ASCII bytes = 1250 approx tokens. chunkSize=100 (400 bytes),
	// overlap=20 (80 bytes) -> step 320 bytes.
	text := strings.Repeat("a", 5000)
	pieces := splitApproxTokens(text, 100, 20)
	if len(pieces) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(pieces))
	}
	// Every piece must be within budget (chunkSize*4 bytes).
	for i, p := range pieces {
		if len(p) == 0 {
			t.Fatalf("piece %d is empty", i)
		}
		if len(p) > 100*4 {
			t.Fatalf("piece %d exceeds budget: %d bytes", i, len(p))
		}
	}
	// Coverage: concatenating all piece starts should cover the text end.
	last := pieces[len(pieces)-1]
	if !strings.HasSuffix(text, last) {
		// The final piece is a suffix of the original text.
		t.Fatal("last piece should be a suffix of the source text")
	}
}

func TestSplitApproxTokens_MultibyteSplitsOnRuneBoundary(t *testing.T) {
	// Chinese chars: 3 bytes each. 1000 chars = 3000 bytes = 750 approx tokens.
	text := strings.Repeat("中", 1000)
	pieces := splitApproxTokens(text, 100, 20)
	for i, p := range pieces {
		if !utf8.ValidString(p) {
			t.Fatalf("piece %d is not valid utf8", i)
		}
	}
}

func TestByteToRuneIdx(t *testing.T) {
	runes := []rune("abc")
	prefix := make([]int, 4)
	for i, r := range runes {
		prefix[i+1] = prefix[i] + utf8.RuneLen(r)
	}
	// prefix = [0,1,2,3]. byteToRuneIdx returns the largest rune-slice
	// index i with prefix[i] <= target (the boundary at/just-before that byte).
	if got := byteToRuneIdx(prefix, 0); got != 0 {
		t.Fatalf("byte 0 -> rune %d", got)
	}
	if got := byteToRuneIdx(prefix, 1); got != 1 {
		t.Fatalf("byte 1 -> rune %d (want 1: start of 'b')", got)
	}
	if got := byteToRuneIdx(prefix, 2); got != 2 {
		t.Fatalf("byte 2 -> rune %d (want 2: start of 'c')", got)
	}
	if got := byteToRuneIdx(prefix, 5); got != 3 {
		t.Fatalf("byte 5 -> rune %d (want 3: clamped to end)", got)
	}
}

func TestApproxTokenChunker_DataBlockPassthrough(t *testing.T) {
	c := NewApproxTokenChunker()
	secs := []document.Section{{
		Content: message.NewDataBlock(message.TypeImage, &message.Source{URL: "x.png"}),
		Source:  "x.png",
	}}
	chunks, err := c.Chunk(secs)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 passthrough chunk, got %d", len(chunks))
	}
	if chunks[0].ChunkIndex != 0 || chunks[0].TotalChunks != 1 {
		t.Fatal("index/total mismatch")
	}
}

func TestApproxTokenChunker_SequentialIndices(t *testing.T) {
	c := &ApproxTokenChunker{ChunkSize: 50, Overlap: 0}
	long := strings.Repeat("abcdefghij", 200) // 2000 bytes
	secs := []document.Section{
		{Content: message.NewTextBlock(long), Source: "a.txt"},
		{Content: message.NewTextBlock("tail"), Source: "a.txt"},
	}
	chunks, err := c.Chunk(secs)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected >=3 chunks, got %d", len(chunks))
	}
	total := len(chunks)
	for i, ch := range chunks {
		if ch.ChunkIndex != i {
			t.Fatalf("chunk %d has index %d", i, ch.ChunkIndex)
		}
		if ch.TotalChunks != total {
			t.Fatalf("chunk %d total %d, want %d", i, ch.TotalChunks, total)
		}
		if ch.Source != "a.txt" {
			t.Fatalf("source not carried: %q", ch.Source)
		}
	}
	// Last chunk is the "tail" text.
	last := chunks[total-1]
	if last.Text() != "tail" {
		t.Fatalf("last chunk text = %q, want 'tail'", last.Text())
	}
}

func TestApproxTokenChunker_DefaultsWhenZero(t *testing.T) {
	c := &ApproxTokenChunker{} // zero values
	secs := []document.Section{{Content: message.NewTextBlock("hi"), Source: "a"}}
	chunks, err := c.Chunk(secs)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestApproxTokenChunker_EmptyInput(t *testing.T) {
	c := NewApproxTokenChunker()
	chunks, err := c.Chunk(nil)
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Fatalf("expected nil chunks, got %v", chunks)
	}
}

// compile-time assertion
var _ Chunker = (*ApproxTokenChunker)(nil)
