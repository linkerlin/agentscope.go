package gateway

import (
	"encoding/json"

	"github.com/linkerlin/agentscope.go/event"
)

// Test-support wire types mirroring gateway/sessionapi's unexported formats,
// kept here so the root-level HTTP tests can build request bodies without
// depending on the sessionapi package internals.

// v2ChatRequest mirrors sessionapi.v2ChatRequest (JSON body of /v2/chat).
type v2ChatRequest struct {
	Text      string `json:"text"`
	SessionID string `json:"session_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}

// wsV2Message mirrors sessionapi.wsV2Message (V2 WebSocket wire format).
type wsV2Message struct {
	Type      string                  `json:"type"`
	Text      string                  `json:"text,omitempty"`
	ConfirmID string                  `json:"confirm_id,omitempty"`
	ReplyID   string                  `json:"reply_id,omitempty"`
	Decisions []event.ConfirmDecision `json:"decisions,omitempty"`
}

// v2Event mirrors sessionapi.v2Event (SSE envelope for V2 streams).
type v2Event struct {
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	ReplyID   string          `json:"reply_id"`
	Payload   json.RawMessage `json:"payload"`
}
