package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
)

// resumeRequest is the expected JSON body for /v2/resume.
type resumeRequest struct {
	SessionID string                  `json:"session_id"`
	ReplyID   string                  `json:"reply_id"`
	ConfirmID string                  `json:"confirm_id"`
	Decisions []event.ConfirmDecision `json:"decisions"`
}

func (s *Server) handleV2Resume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	v2, ok := s.agent.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	var req resumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("parse error: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if req.ConfirmID == "" {
		http.Error(w, "confirm_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := s.sessionState.Resume(ctx, req.SessionID, v2,
		event.NewUserConfirmResult(req.ReplyID, req.ConfirmID, req.Decisions)); err != nil {
		if errors.Is(err, ErrStorageNotAvailable) {
			http.Error(w, "session state persistence not available", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, fmt.Sprintf("resume failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "resumed",
		"session": req.SessionID,
		"reply":   req.ReplyID,
	})
}
