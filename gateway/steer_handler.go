package gateway

import (
	"net/http"
)

type steerRequest struct {
	Text string `json:"text"`
}

// handleV2Steer injects a user message into an active run (mid-turn steering).
// POST /v2/sessions/{session_id}/steer
func (s *Server) handleV2Steer(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session manager not configured"})
		return
	}
	sessionID := r.PathValue("session_id")
	var req steerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "text is required"})
		return
	}
	if err := s.sessionMgr.Steer(sessionID, req.Text); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleV2Interrupt terminates the active run for a session (agent Interrupt
// + context cancel). POST /v2/sessions/{session_id}/interrupt
func (s *Server) handleV2Interrupt(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session manager not configured"})
		return
	}
	sessionID := r.PathValue("session_id")
	if !s.sessionMgr.Terminate(sessionID) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no active run for session " + sessionID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
