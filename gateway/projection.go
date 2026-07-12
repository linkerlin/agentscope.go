package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/messagebus"
)

// SessionProjection lets one session project UI cards (e.g. a worker's pending
// human-in-the-loop request) onto another session's view, stored in the
// CoordBus registry under a per-target namespace. Mirrors Python agentscope's
// session projection feed.
//
// It degrades gracefully: if the attached bus does not implement CoordBus,
// every method is a no-op (returns nil), so single-process LocalBus-only
// deployments work without extra wiring.
type SessionProjection struct {
	coord messagebus.CoordBus
}

// NewSessionProjection builds a projection store over bus. Returns a store that
// no-ops if bus is not a CoordBus.
func NewSessionProjection(bus messagebus.Bus) *SessionProjection {
	return &SessionProjection{coord: messagebus.AsCoordBus(bus)}
}

// Project stores a UI card (key → payload) onto the target session's projection
// feed. Overwrites any existing card with the same key.
func (p *SessionProjection) Project(ctx context.Context, targetSessionID, key string, payload []byte) error {
	if p.coord == nil {
		return nil
	}
	return p.coord.RegistrySet(ctx, messagebus.Keys.ProjectionNS(targetSessionID), key, payload)
}

// Clear removes one projected card from the target session.
func (p *SessionProjection) Clear(ctx context.Context, targetSessionID, key string) error {
	if p.coord == nil {
		return nil
	}
	return p.coord.RegistryDelete(ctx, messagebus.Keys.ProjectionNS(targetSessionID), key)
}

// List returns all cards currently projected onto the target session.
func (p *SessionProjection) List(ctx context.Context, targetSessionID string) (map[string][]byte, error) {
	if p.coord == nil {
		return map[string][]byte{}, nil
	}
	return p.coord.RegistryList(ctx, messagebus.Keys.ProjectionNS(targetSessionID))
}

// projectionHandlers exposes SessionProjection over HTTP for the Studio / frontend.
func (s *Server) RegisterProjectionRoutes() {
	sp := s.sessionProjection()
	if sp == nil {
		return
	}
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/projections", s.requireAuth(s.handleListProjections))
	s.mux.HandleFunc("DELETE /api/v1/sessions/{id}/projections/{key}", s.requireAuth(s.handleClearProjection))
}

// sessionProjection returns the projection store, auto-built from the bus.
func (s *Server) sessionProjection() *SessionProjection {
	if s.messageBus == nil {
		return nil
	}
	return NewSessionProjection(s.messageBus)
}

func (s *Server) handleListProjections(w http.ResponseWriter, r *http.Request) {
	sp := s.sessionProjection()
	sid := r.PathValue("id")
	cards, err := sp.List(r.Context(), sid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_id": sid, "projections": cards})
}

func (s *Server) handleClearProjection(w http.ResponseWriter, r *http.Request) {
	sp := s.sessionProjection()
	sid := r.PathValue("id")
	key := r.PathValue("key")
	if err := sp.Clear(r.Context(), sid, key); err != nil {
		http.Error(w, fmt.Sprintf("clear failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
