// Package loader provides document loading utilities for RAG pipelines.
// It defines a common Loader interface and implementations for text files
// and directory traversal.
package loader

import (
	"context"
)

// Document represents a loaded document with its content and metadata.
type Document struct {
	Content  string
	Metadata map[string]any
}

// Loader loads documents from a source (file path, URL, etc.).
type Loader interface {
	Load(ctx context.Context, source string) ([]Document, error)
}
