// Package chunker splits parsed Sections into indexable Chunks. A Chunker
// never merges content across two Sections — Section is the hard boundary.
//
// Aligned with Python AgentScope rag._chunker (ChunkerBase +
// ApproxTokenChunker). Token counts are approximated as
// len(text utf-8 bytes)/4, avoiding a hard tokenizer dependency.
package chunker

import (
	"sort"
	"unicode/utf8"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/rag/document"
)

// Chunker slices []Section into []Chunk. Each Chunk is one vector-store record.
type Chunker interface {
	Chunk(sections []document.Section) ([]document.Chunk, error)
}

// ApproxTokenChunker slices text Sections into pieces of at most ChunkSize
// approximate tokens, with Overlap approximate tokens shared between
// consecutive pieces. Sections carrying a DataBlock are passed through
// unchanged as a single chunk. Chunks never span two Sections.
type ApproxTokenChunker struct {
	// ChunkSize is the max approximate tokens per chunk (default 512).
	ChunkSize int
	// Overlap is the shared approximate tokens between neighbours (default 50).
	Overlap int
}

// NewApproxTokenChunker returns a chunker with sensible defaults.
func NewApproxTokenChunker() *ApproxTokenChunker {
	return &ApproxTokenChunker{ChunkSize: 512, Overlap: 50}
}

// Chunk splits each Section into one or more Chunks with sequential,
// document-wide indices and a correct TotalChunks per source.
func (c *ApproxTokenChunker) Chunk(sections []document.Section) ([]document.Chunk, error) {
	size, overlap := c.ChunkSize, c.Overlap
	if size <= 0 {
		size = 512
	}
	if overlap < 0 {
		overlap = 0
	}

	// First pass: compute the chunk substrings per section (without indices)
	// so we can assign a single accurate TotalChunks across the document.
	type sectionChunks struct {
		source   string
		metadata map[string]any
		pieces   []message.ContentBlock
	}
	staged := make([]sectionChunks, 0, len(sections))
	total := 0
	for _, sec := range sections {
		sc := sectionChunks{source: sec.Source, metadata: sec.Metadata}
		if tb, ok := sec.Content.(*message.TextBlock); ok {
			pieces := splitApproxTokens(tb.Text, size, overlap)
			for _, p := range pieces {
				sc.pieces = append(sc.pieces, message.NewTextBlock(p))
			}
		} else {
			// DataBlock (or any non-text): pass through unchanged.
			sc.pieces = []message.ContentBlock{sec.Content}
		}
		staged = append(staged, sc)
		total += len(sc.pieces)
	}
	if total == 0 {
		return nil, nil
	}

	out := make([]document.Chunk, 0, total)
	idx := 0
	for _, sc := range staged {
		for _, content := range sc.pieces {
			out = append(out, document.Chunk{
				Content:     content,
				Source:      sc.source,
				ChunkIndex:  idx,
				TotalChunks: total,
				Metadata:    sc.metadata,
			})
			idx++
		}
	}
	return out, nil
}

// splitApproxTokens slices text into substrings. Approx token count of a
// string is len(utf8 bytes)/4. Each piece is at most chunkSize approx tokens;
// consecutive pieces overlap by `overlap` approx tokens.
func splitApproxTokens(text string, chunkSize, overlap int) []string {
	budget := chunkSize * 4 // bytes per chunk
	step := (chunkSize - overlap) * 4
	if step <= 0 {
		step = budget
	}
	if budget <= 0 {
		return []string{text}
	}

	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}
	// prefix[i] = total utf8 bytes for runes[0:i].
	prefix := make([]int, n+1)
	for i, r := range runes {
		prefix[i+1] = prefix[i] + utf8.RuneLen(r)
	}
	totalBytes := prefix[n]
	// Whole text fits in one chunk.
	if totalBytes <= budget {
		return []string{text}
	}

	var chunks []string
	startByte := 0
	for startByte < totalBytes {
		endByte := startByte + budget
		if endByte > totalBytes {
			endByte = totalBytes
		}
		si := byteToRuneIdx(prefix, startByte)
		ei := byteToRuneIdx(prefix, endByte)
		if ei < si {
			ei = si
		}
		chunks = append(chunks, string(runes[si:ei]))
		if endByte >= totalBytes {
			break
		}
		startByte += step
		// Guard against zero-progress if step rounded below one rune.
		if byteToRuneIdx(prefix, startByte) <= si {
			startByte = prefix[si+1]
		}
	}
	if len(chunks) == 0 {
		chunks = []string{text}
	}
	return chunks
}

// byteToRuneIdx returns the largest rune index i with prefix[i] <= target.
func byteToRuneIdx(prefix []int, target int) int {
	if target <= 0 {
		return 0
	}
	// sort.Search returns the smallest i satisfying the func.
	idx := sort.Search(len(prefix), func(i int) bool { return prefix[i] > target })
	if idx <= 0 {
		return 0
	}
	return idx - 1
}
