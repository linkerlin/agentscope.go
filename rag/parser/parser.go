// Package parser provides document parsers and a media-type-routed registry
// for the RAG indexing pipeline. A Parser turns a raw file into []Section;
// the Registry routes by the upload's media type.
//
// Aligned with Python AgentScope rag._parser (ParserBase + registry).
package parser

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/linkerlin/agentscope.go/rag/document"
)

// Parser converts a raw byte stream into one or more Sections.
type Parser interface {
	// Parse reads from r and returns the natural Sections of the document.
	// source is the filename, carried into each Section for citation.
	Parse(ctx context.Context, r io.Reader, source string) ([]document.Section, error)
	// SupportedMediaTypes lists the MIME types this parser handles (e.g.
	// "text/plain", "application/pdf"). The Registry routes by these.
	SupportedMediaTypes() []string
}

// Registry routes an upload to a Parser by media type. Thread-safe.
type Registry struct {
	mu     sync.RWMutex
	byType map[string]Parser
}

// NewRegistry builds a registry pre-populated with the given parsers.
func NewRegistry(parsers ...Parser) *Registry {
	r := &Registry{byType: make(map[string]Parser)}
	for _, p := range parsers {
		r.Register(p)
	}
	return r
}

// Register adds (or replaces) a parser for each of its supported types.
func (r *Registry) Register(p Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, mt := range p.SupportedMediaTypes() {
		r.byType[mt] = p
	}
}

// Get returns the parser registered for mediaType, if any.
func (r *Registry) Get(mediaType string) (Parser, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byType[mediaType]
	return p, ok
}

// Parse is a convenience: resolve the parser for mediaType and run it.
func (r *Registry) Parse(ctx context.Context, mediaType, source string, rd io.Reader) ([]document.Section, error) {
	p, ok := r.Get(mediaType)
	if !ok {
		return nil, fmt.Errorf("parser: no parser registered for media type %q", mediaType)
	}
	return p.Parse(ctx, rd, source)
}
