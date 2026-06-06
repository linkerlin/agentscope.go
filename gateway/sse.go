package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// v2Event is the SSE payload for V2 AgentEvent streaming.
type v2Event struct {
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	ReplyID   string          `json:"reply_id"`
	Payload   json.RawMessage `json:"payload"`
}

// v2ChatRequest is the expected JSON body for /v2/chat/stream.
type v2ChatRequest struct {
	Text      string `json:"text"`
	SessionID string `json:"session_id,omitempty"`
}

func parseV2ChatRequest(body json.RawMessage) (*v2ChatRequest, error) {
	var req v2ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	if req.Text == "" {
		return nil, fmt.Errorf("text is required")
	}
	return &req, nil
}

func (s *Server) handleV2ChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readAllAndClose(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req, err := parseV2ChatRequest(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v2, ok := s.agent.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
	ch, err := v2.ReplyStream(r.Context(), msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("reply stream error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendEvent := func(ev event.AgentEvent) bool {
		payload, _ := json.Marshal(ev)
		data, _ := json.Marshal(v2Event{
			EventType: ev.EventType(),
			Timestamp: ev.Timestamp().Format("2006-01-02T15:04:05.000Z"),
			ReplyID:   ev.ReplyID(),
			Payload:   payload,
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return true
	}

	for ev := range ch {
		if ev == nil {
			continue
		}

		// Persist snapshot when the agent suspends waiting for user confirmation.
		if _, suspended := ev.(*event.RequireUserConfirmEvent); suspended && req.SessionID != "" {
			if err := s.sessionState.SaveSnapshot(r.Context(), req.SessionID, v2); err != nil {
				// Emit an error event but keep the stream alive so the client sees the suspend event.
				errEv := event.NewError(ev.ReplyID(), fmt.Errorf("save snapshot failed: %w", err))
				_ = sendEvent(errEv)
			}
		}

		if !sendEvent(ev) {
			break
		}

		// Clean up snapshot when the reply completes successfully.
		if _, isEnd := ev.(*event.ReplyEndEvent); isEnd && req.SessionID != "" {
			_ = s.sessionState.DeleteSnapshot(r.Context(), req.SessionID)
		}
	}

	// Send a terminal done event.
	data, _ := json.Marshal(v2Event{EventType: "done", Timestamp: "", ReplyID: "", Payload: []byte("{}")})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
