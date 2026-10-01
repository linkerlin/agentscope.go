// Package sessionapi holds the session HTTP face of the gateway (streamable
// HTTP, WebSocket, steer/interrupt, resume, status) as the second
// registration-functionized cluster after gateway/kbapi: the Handlers type
// depends only on narrow interfaces and injected functions — never on the
// gateway root package — and mounts its own routes via Register.
package sessionapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// HeaderAgentSessionID carries the session id between client and server.
	HeaderAgentSessionID = "Agent-Session-Id"
	headerAccept         = "Accept"
)

// v2Event is the SSE payload for V2 AgentEvent streaming.
type v2Event struct {
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	ReplyID   string          `json:"reply_id"`
	Payload   json.RawMessage `json:"payload"`
}

// v2ChatRequest is the expected JSON body for /v2/chat and /v2/chat/stream.
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

// chatRequest mirrors the legacy /chat wire format (kept local so this
// package does not depend on the gateway root).
type chatRequest struct {
	Text string `json:"text"`
}

// streamEvent is the legacy V1 delta wire format.
type streamEvent struct {
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// readAllAndClose reads the request body under maxBody (18.11): bodies past
// the cap stop reading at the boundary and return *http.MaxBytesError (the
// handler maps it to 413). A zero maxBody means uncapped (tests, embedded
// reuse).
func readAllAndClose(w http.ResponseWriter, body io.ReadCloser, maxBody int64) ([]byte, error) {
	defer body.Close()
	if maxBody > 0 {
		body = http.MaxBytesReader(w, body, maxBody)
	}
	return io.ReadAll(body)
}
