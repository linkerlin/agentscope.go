package rag

// Document represents a parsed document with extracted text and metadata.
type Document struct {
	Text     string
	Metadata map[string]any
}
