// Package document defines the transient in-memory data structures that
// flow between the two RAG indexing-pipeline stages.
//
// A Parser produces []Section; a Chunker consumes []Section and produces
// []Chunk. Sections are hard boundaries (a PDF page, a PPTX slide, an
// embedded image): a Chunker never merges content across two Sections.
//
// Aligned with Python AgentScope rag._document (Section/Chunk) for
// cross-language JSON compatibility.
package document

import "github.com/linkerlin/agentscope.go/message"

// Section is one natural region of a parsed source file, produced by a
// Parser. It is a hard boundary for chunking: no Chunk spans two Sections.
//
// Granularity is format-specific — PDF: one per page (+ images);
// PPTX: one per slide; TXT/image: one for the whole file.
type Section struct {
	// Content is the section payload: *message.TextBlock for text,
	// *message.DataBlock for multimodal content (images, etc.).
	Content message.ContentBlock
	// Source is the source filename (e.g. "report.pdf"), carried through
	// to every downstream Chunk and into vector-store metadata for citation.
	Source string
	// Metadata holds format-specific parser output (e.g. {"page": 3},
	// {"slide": 2}). Not part of any pipeline contract — passed through
	// verbatim. Each Chunk inherits this from its parent Section.
	Metadata map[string]any
}

// Chunk is the final indexable unit produced by a Chunker. Each Chunk
// corresponds to one record in the vector store.
type Chunk struct {
	// Content is the chunk payload (sliced text, or a DataBlock passed
	// through unchanged).
	Content message.ContentBlock
	// Source is the source filename, inherited from the parent Section.
	Source string
	// ChunkIndex is the 0-based index of this chunk within the document,
	// sequential across all sections of the same source. Enables
	// "expand context around a hit" at query time.
	ChunkIndex int
	// TotalChunks is the total number of chunks produced from the same
	// source file, bounding context-expansion range.
	TotalChunks int
	// Metadata is format-specific metadata inherited from the parent Section.
	Metadata map[string]any
}

// Text returns the section's text content if it is a TextBlock, else "".
func (s Section) Text() string {
	if tb, ok := s.Content.(*message.TextBlock); ok {
		return tb.Text
	}
	return ""
}

// IsData reports whether the section carries a DataBlock (multimodal).
func (s Section) IsData() bool { _, ok := s.Content.(*message.DataBlock); return ok }

// Text returns the chunk's text content if it is a TextBlock, else "".
func (c Chunk) Text() string {
	if tb, ok := c.Content.(*message.TextBlock); ok {
		return tb.Text
	}
	return ""
}
