// Package kbapi holds the knowledge-base HTTP handlers, extracted from the
// gateway root package as the pattern validation for 16.2 (handlers own a
// narrow dependency surface and register themselves; the gateway root only
// calls Register). The public surface stays in gateway via type alias.
package kbapi

import (
	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/index"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

// KBService bundles the components needed to serve the knowledge-base HTTP
// API: a KB manager, blob store, parser registry, chunker, and a pipeline
// worker.
type KBService struct {
	Manager *kb.KBManager
	Blob    blob.BlobStore
	Parsers *parser.Registry
	Chunker chunker.Chunker
	Worker  *index.Worker
}

// NewKBService builds a KBService from the core components, auto-creating the
// pipeline Worker.
func NewKBService(mgr *kb.KBManager, bs blob.BlobStore, parsers *parser.Registry, ch chunker.Chunker) *KBService {
	svc := &KBService{Manager: mgr, Blob: bs, Parsers: parsers, Chunker: ch}
	svc.Worker = &index.Worker{Blob: bs, Parsers: parsers, Chunker: ch, Manager: mgr}
	return svc
}
