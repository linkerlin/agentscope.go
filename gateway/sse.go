package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	AgentID   string `json:"agent_id,omitempty"`
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
	s.handleV2ChatStreamLegacy(w, r)
}
