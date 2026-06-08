package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// Streamable HTTP transport for Agent Service (MCP 2025-03-26 inspired):
// single endpoint /v2/chat supports POST (send message), GET (subscribe/reconnect),
// and DELETE (terminate active run).
//
// Clients SHOULD send:
//   Accept: application/json, text/event-stream
//   Agent-Session-Id: <session>   (optional; body session_id also accepted)
//
// POST returns Content-Type: text/event-stream by default (event stream).
// GET replays buffered events for an active or recently completed session run.

const (
	HeaderAgentSessionID = "Agent-Session-Id"
	headerAccept         = "Accept"
)

type chatStreamParams struct {
	sessionID string
	agentID   string
	text      string
	useAGUI   bool
	strictAccept bool
}

// handleV2Chat is the Streamable HTTP endpoint (POST + GET + DELETE).
func (s *Server) handleV2Chat(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleV2ChatPost(w, r, chatStreamParams{strictAccept: true})
	case http.MethodGet:
		s.handleV2ChatGet(w, r)
	case http.MethodDelete:
		s.handleV2ChatDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleV2ChatStreamLegacy keeps the deprecated SSE-only path for older clients.
func (s *Server) handleV2ChatStreamLegacy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleV2ChatPost(w, r, chatStreamParams{strictAccept: false})
}

func (s *Server) handleV2ChatPost(w http.ResponseWriter, r *http.Request, opts chatStreamParams) {
	if opts.strictAccept && !acceptsStreamableHTTP(r) {
		http.Error(w, "Accept must include application/json and text/event-stream", http.StatusNotAcceptable)
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

	sessionID := firstNonEmpty(req.SessionID, r.Header.Get(HeaderAgentSessionID))
	params := chatStreamParams{
		sessionID: sessionID,
		agentID:   req.AgentID,
		text:      req.Text,
		useAGUI:   useAGUIProtocol(r),
	}

	a, err := s.resolveAgent(r, params.agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v2, ok := a.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent(injectOffloadHints(s, params.sessionID, params.text)).Build()
	ch, err := s.startAgentEventStream(r, a, v2, params.sessionID, msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("reply stream error: %v", err), http.StatusInternalServerError)
		return
	}

	if params.sessionID != "" {
		w.Header().Set(HeaderAgentSessionID, params.sessionID)
	}

	if wantsJSONOnly(r) {
		s.writeChatJSON(w, ch, params)
		return
	}

	s.writeChatSSE(w, r, ch, v2, params)
}

func (s *Server) handleV2ChatGet(w http.ResponseWriter, r *http.Request) {
	if !acceptsStreamableHTTP(r) && r.Header.Get(headerAccept) != "" {
		http.Error(w, "Accept must include text/event-stream", http.StatusNotAcceptable)
		return
	}

	sessionID := firstNonEmpty(r.URL.Query().Get("session_id"), r.Header.Get(HeaderAgentSessionID))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if s.sessionMgr == nil {
		http.Error(w, "session manager not configured", http.StatusServiceUnavailable)
		return
	}

	params := chatStreamParams{
		sessionID: sessionID,
		useAGUI:   useAGUIProtocol(r),
	}

	w.Header().Set(HeaderAgentSessionID, sessionID)
	ch := s.sessionMgr.Subscribe(sessionID)

	// Subscribe may return empty closed channel; still emit terminal done.
	s.writeChatSSE(w, r, ch, nil, params)
}

func (s *Server) handleV2ChatDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := firstNonEmpty(r.URL.Query().Get("session_id"), r.Header.Get(HeaderAgentSessionID))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if s.sessionMgr == nil {
		http.Error(w, "session manager not configured", http.StatusServiceUnavailable)
		return
	}

	terminated := s.sessionMgr.Terminate(sessionID)
	if !terminated {
		http.Error(w, "no active run for session", http.StatusNotFound)
		return
	}

	s.sessionMgr.ClearCompleted(sessionID)
	if s.sessionState != nil {
		_ = s.sessionState.DeleteSnapshot(r.Context(), sessionID)
	}

	w.Header().Set(HeaderAgentSessionID, sessionID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startAgentEventStream(
	r *http.Request,
	a agent.Agent,
	v2 agent.V2Agent,
	sessionID string,
	msg *message.Msg,
) (<-chan event.AgentEvent, error) {
	if s.sessionMgr != nil && sessionID != "" {
		return s.sessionMgr.Run(r.Context(), sessionID, a, msg)
	}
	return v2.ReplyStream(r.Context(), msg)
}

func (s *Server) writeChatSSE(
	w http.ResponseWriter,
	r *http.Request,
	ch <-chan event.AgentEvent,
	v2 agent.V2Agent,
	params chatStreamParams,
) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	var aguiConv *DefaultAGUIConverter
	if params.useAGUI {
		aguiConv = NewDefaultAGUIConverter()
	}
	opts := AGUIConvertOptions{ThreadID: params.sessionID}

	sendEvent := func(ev event.AgentEvent) bool {
		data, err := EncodeStreamEvent(ev, opts, params.useAGUI, aguiConv)
		if err != nil {
			return false
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return true
	}

	for ev := range ch {
		if ev == nil {
			continue
		}

		if v2 != nil && s.sessionState != nil {
			if _, suspended := ev.(*event.RequireUserConfirmEvent); suspended && params.sessionID != "" {
				if err := s.sessionState.SaveSnapshot(r.Context(), params.sessionID, v2); err != nil {
					errEv := event.NewError(ev.ReplyID(), fmt.Errorf("save snapshot failed: %w", err))
					_ = sendEvent(errEv)
				}
			}
		}

		if !sendEvent(ev) {
			break
		}

		if params.sessionID != "" && s.sessionState != nil {
			if _, isEnd := ev.(*event.ReplyEndEvent); isEnd {
				_ = s.sessionState.DeleteSnapshot(r.Context(), params.sessionID)
			}
		}
	}

	writeStreamDone(w, flusher, params.useAGUI)
}

func writeStreamDone(w http.ResponseWriter, flusher http.Flusher, useAGUI bool) {
	if useAGUI {
		data, _ := json.Marshal(map[string]any{"type": "STREAM_DONE"})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	} else {
		data, _ := json.Marshal(v2Event{EventType: "done", Timestamp: "", ReplyID: "", Payload: []byte("{}")})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()
}

func (s *Server) writeChatJSON(w http.ResponseWriter, ch <-chan event.AgentEvent, params chatStreamParams) {
	events := make([]json.RawMessage, 0, 32)
	var aguiConv *DefaultAGUIConverter
	if params.useAGUI {
		aguiConv = NewDefaultAGUIConverter()
	}
	opts := AGUIConvertOptions{ThreadID: params.sessionID}

	for ev := range ch {
		if ev == nil {
			continue
		}
		data, err := EncodeStreamEvent(ev, opts, params.useAGUI, aguiConv)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		events = append(events, json.RawMessage(data))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": params.sessionID,
		"events":     events,
	})
}

func acceptsStreamableHTTP(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get(headerAccept))
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "application/json") && strings.Contains(accept, "text/event-stream")
}

func wantsJSONOnly(r *http.Request) bool {
	if r.URL.Query().Get("stream") == "false" {
		return true
	}
	accept := strings.ToLower(r.Header.Get(headerAccept))
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/event-stream")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
