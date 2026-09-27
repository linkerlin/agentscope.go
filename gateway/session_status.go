package gateway

import (
	"net/http"
)

// Session status endpoint (18.2): GET /api/v1/sessions/{id}/status resolves
// the session's lifecycle state — running / parked / idle / unknown — from
// the session coordinator's data plane (run-lock registry marker, event log
// tail, agent suspension state), falling back to the in-process manager.
func (s *Server) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.checkSessionAccess(w, r, id) {
		return
	}
	status := StatusUnknown
	switch {
	case s.sessionCoord != nil:
		status = s.sessionCoord.Status(r.Context(), id)
	case s.sessionMgr != nil:
		// No coordinator wired: degrade to local-only resolution.
		switch {
		case s.sessionMgr.IsActive(id):
			if s.sessionMgr.Suspended(id) {
				status = StatusParked
			} else {
				status = StatusRunning
			}
		case s.sessionMgr.HasCompleted(id):
			status = StatusIdle
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": id,
		"status":     status,
	})
}

// registerSessionStatusRoutes mounts the status route next to the other
// session endpoints.
func (s *Server) registerSessionStatusRoutes() {
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/status", s.requireAuth(s.handleSessionStatus))
}
