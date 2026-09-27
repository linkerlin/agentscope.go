package gateway

import (
	"github.com/linkerlin/agentscope.go/gateway/kbapi"
	"github.com/linkerlin/agentscope.go/rag/blob"
	"github.com/linkerlin/agentscope.go/rag/chunker"
	"github.com/linkerlin/agentscope.go/rag/kb"
	"github.com/linkerlin/agentscope.go/rag/parser"
)

// The knowledge-base HTTP API lives in gateway/kbapi (16.2 pattern
// validation): handler logic owns a narrow dependency surface and registers
// itself via kbapi.Handlers.Register. The public surface stays here —
// KBService is a type alias and Server keeps its KB attachment points — so
// AppConfig and external callers are unaffected.

// KBService is the knowledge-base service powering the KB HTTP API.
// Alias of kbapi.KBService; build via NewKBService.
type KBService = kbapi.KBService

// NewKBService builds a KBService from the core components, auto-creating the
// pipeline Worker.
func NewKBService(mgr *kb.KBManager, bs blob.BlobStore, parsers *parser.Registry, ch chunker.Chunker) *KBService {
	return kbapi.NewKBService(mgr, bs, parsers, ch)
}

// WithKBService attaches a knowledge-base service for KB HTTP endpoints.
func (s *Server) WithKBService(svc *KBService) *Server {
	s.kbService = svc
	return s
}

// RegisterKBRoutes registers the knowledge-base CRUD + upload + search
// routes. No-op if no KBService is attached. Route registration stays in the
// gateway root: it delegates to kbapi.Handlers.Register with this server's
// mux and auth wrapper.
func (s *Server) RegisterKBRoutes() {
	if s.kbService == nil {
		return
	}
	kbapi.NewHandlers(s.kbService).Register(s.mux, s.requireAuth)
}
