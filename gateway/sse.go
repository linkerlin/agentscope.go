package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// v2Event is the SSE payload for V2 AgentEvent streaming.
type v2Event struct {
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	ReplyID   string          `json:"reply_id"`
	Payload   json.RawMessage `json:"payload"`
}

func (s *Server) handleV2ChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, err := parseChatRequest(r.Body)
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

	for ev := range ch {
		if ev == nil {
			continue
		}
		payload, _ := json.Marshal(ev)
		data, _ := json.Marshal(v2Event{
			EventType: ev.EventType(),
			Timestamp: ev.Timestamp().Format("2006-01-02T15:04:05.000Z"),
			ReplyID:   ev.ReplyID(),
			Payload:   payload,
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Send a terminal done event.
	data, _ := json.Marshal(v2Event{EventType: "done", Timestamp: "", ReplyID: "", Payload: []byte("{}")})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
