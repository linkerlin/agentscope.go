package sessionapi

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

func (h *Handlers) handleV2Resume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	v2, ok := h.d.Agent.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	body, err := readAllAndClose(w, r.Body, h.d.MaxBodyBytes)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, fmt.Sprintf("read error: %v", err), http.StatusBadRequest)
		return
	}
	var req resumeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("parse error: %v", err), http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	// Ownership is checked before body validation details (22.2): unknown or
	// foreign sessions get a uniform 404 regardless of what else the request
	// contains.
	if !h.checkSessionAccess(w, r, req.SessionID) {
		return
	}
	if req.ConfirmID == "" {
		http.Error(w, "confirm_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	// nil State (no storage) keeps the old nil-receiver semantics: in-memory
	// resume via InjectEvent.
	var resumeErr error
	if h.d.State != nil {
		resumeErr = h.d.State.Resume(ctx, req.SessionID, v2,
			event.NewUserConfirmResult(req.ReplyID, req.ConfirmID, req.Decisions))
	} else {
		resumeErr = v2.InjectEvent(ctx, event.NewUserConfirmResult(req.ReplyID, req.ConfirmID, req.Decisions))
	}
	if resumeErr != nil {
		if errors.Is(resumeErr, ErrStorageNotAvailable) {
			http.Error(w, "session state persistence not available", http.StatusServiceUnavailable)
			return
		}
		if errors.Is(resumeErr, ErrResumeAlreadyExecuting) {
			// 23.2 idempotency: the command was already delivered and the
			// resumed work is running — refuse the duplicate (at-most-once).
			http.Error(w, `{"error":"resume already executing","status":"executing"}`, http.StatusConflict)
			return
		}
		if errors.Is(resumeErr, ErrResumeNotDelivered) {
			// Command persisted, but this replica holds no live waiter for
			// it. Notify the worker tier (wakeup) so the replica that can
			// consume the command does (18.5); the command itself stays on
			// the snapshot.
			if h.d.OnResumeNotified != nil {
				h.d.OnResumeNotified(req.SessionID)
			}
			http.Error(w, `{"error":"resume not delivered","status":"pending"}`, http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("resume failed: %v", resumeErr), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "resumed",
		"session": req.SessionID,
		"reply":   req.ReplyID,
	})
}
