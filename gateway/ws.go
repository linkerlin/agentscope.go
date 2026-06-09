package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for demo; tighten in production
	},
}

const (
	heartbeatInterval = 30 * time.Second
	heartbeatTimeout  = 10 * time.Second
)

func (s *Server) handleChatWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	room := r.URL.Query().Get("room")

	ws := &wsSession{
		id:       sessionID,
		room:     room,
		conn:     conn,
		lastPing: time.Now(),
	}
	s.registerSession(ws)
	defer func() {
		s.unregisterSession(ws)
		ws.close()
	}()

	// Heartbeat goroutine
	stopHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ws.writeControl(websocket.PingMessage, []byte{}, time.Now().Add(heartbeatTimeout)); err != nil {
					return
				}
			case <-stopHeartbeat:
				return
			}
		}
	}()

	// Message read loop
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var req chatRequest
		if err := json.Unmarshal(data, &req); err != nil {
			_ = ws.writeJSON(map[string]string{"error": fmt.Sprintf("parse error: %v", err)})
			continue
		}
		if req.Text == "" {
			_ = ws.writeJSON(map[string]string{"error": "text is required"})
			continue
		}

		msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
		ch, err := s.agent.CallStream(r.Context(), msg)
		if err != nil {
			_ = ws.writeJSON(map[string]string{"error": fmt.Sprintf("stream error: %v", err)})
			continue
		}

		for chunk := range ch {
			if chunk == nil {
				continue
			}
			ev := streamEvent{Delta: chunk.GetTextContent()}
			if err := ws.writeJSON(ev); err != nil {
				break
			}
		}
		if err := ws.writeJSON(streamEvent{Done: true}); err != nil {
			break
		}
	}
	close(stopHeartbeat)
}

// wsV2Message is the wire format for V2 WebSocket messages.
type wsV2Message struct {
	Type      string                `json:"type"`
	Text      string                `json:"text,omitempty"`
	ConfirmID string                `json:"confirm_id,omitempty"`
	ReplyID   string                `json:"reply_id,omitempty"`
	Decisions []event.ConfirmDecision `json:"decisions,omitempty"`
}

// handleChatWSV2 serves the V2 WebSocket endpoint that streams AgentEvents.
// It supports suspend-resume: when a RequireUserConfirmEvent is emitted,
// the stream pauses and the AgentState is saved to Storage (if configured).
// The client must send a "resume" message with decisions to continue.
func (s *Server) handleChatWSV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	sessionID := r.URL.Query().Get("session")
	a, err := s.resolveAgentForRequest(r, agentID, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v2, ok := a.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	room := r.URL.Query().Get("room")

	ws := &wsSession{
		id:       sessionID,
		room:     room,
		conn:     conn,
		lastPing: time.Now(),
	}
	s.registerSession(ws)
	defer func() {
		s.unregisterSession(ws)
		ws.close()
	}()

	// Heartbeat goroutine
	stopHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ws.writeControl(websocket.PingMessage, []byte{}, time.Now().Add(heartbeatTimeout)); err != nil {
					return
				}
			case <-stopHeartbeat:
				return
			}
		}
	}()
	defer close(stopHeartbeat)

	var (
		streamCtx    context.Context
		streamCancel context.CancelFunc
		evForward    chan event.AgentEvent
	)

	useAGUI := useAGUIProtocol(r)
	var aguiConv *DefaultAGUIConverter
	if useAGUI {
		aguiConv = NewDefaultAGUIConverter()
	}

	startStream := func(text string) {
		if streamCancel != nil {
			streamCancel()
		}
		if evForward != nil {
			for len(evForward) > 0 {
				<-evForward
			}
		} else {
			evForward = make(chan event.AgentEvent, 64)
		}

		streamCtx, streamCancel = context.WithCancel(r.Context())
		streamCtx = s.enrichContextWithWorkspaceTools(streamCtx, agentID, sessionID)
		msg := message.NewMsg().Role(message.RoleUser).TextContent(injectOffloadHints(s, sessionID, text)).Build()

		var evCh <-chan event.AgentEvent
		var err error
		if s.sessionMgr != nil && sessionID != "" {
			evCh, err = s.sessionMgr.Run(streamCtx, sessionID, a, msg)
		} else {
			evCh, err = v2.ReplyStream(streamCtx, msg)
		}
		if err != nil {
			_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(fmt.Sprintf(`{"error":"%v"}`, err))})
			return
		}

		go func() {
			for ev := range evCh {
				select {
				case evForward <- ev:
				case <-streamCtx.Done():
					return
				}
			}
		}()
	}

	needsStreamStart := true

	for {
		if needsStreamStart {
			// Check for reconnect resume before waiting for a new chat message.
			if sessionID != "" && s.sessionState.HasPendingSnapshot(r.Context(), sessionID) {
				if _, err := s.sessionState.LoadSnapshot(r.Context(), sessionID, v2); err == nil {
					// Agent will detect the suspended state and enter resume path automatically.
					startStream("resume")
					needsStreamStart = false
					continue
				}
			}

			// Wait for a chat message to start a new stream.
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var wsMsg wsV2Message
			_ = json.Unmarshal(data, &wsMsg)
			if wsMsg.Type == "" {
				// Fallback: try old chatRequest format for backward compatibility.
				var oldReq chatRequest
				if err := json.Unmarshal(data, &oldReq); err == nil && oldReq.Text != "" {
					wsMsg = wsV2Message{Type: "chat", Text: oldReq.Text}
				}
			}
			if wsMsg.Type == "chat" && wsMsg.Text != "" {
				startStream(wsMsg.Text)
				needsStreamStart = false
			} else {
				_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(`{"error":"expected chat message"}`)})
			}
			continue
		}

		// Consume events from the forward channel.
		ev := <-evForward
		if ev == nil {
			continue
		}

		if _, suspended := ev.(*event.RequireUserConfirmEvent); suspended {
			// Save snapshot for resume (including reconnect resume).
			if err := s.sessionState.SaveSnapshot(streamCtx, sessionID, v2); err != nil {
				_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(fmt.Sprintf(`{"error":"save snapshot failed: %v"}`, err))})
			}
			if err := writeV2Event(ws, ev, sessionID, useAGUI, aguiConv); err != nil {
				break
			}
			// Wait for resume message.
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					goto done
				}
				var wsMsg wsV2Message
				if err := json.Unmarshal(data, &wsMsg); err != nil {
					_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(`{"error":"parse error"}`)})
					continue
				}
				if wsMsg.Type == "resume" {
					resumeErr := s.sessionState.Resume(streamCtx, sessionID, v2,
						event.NewUserConfirmResult(wsMsg.ReplyID, wsMsg.ConfirmID, wsMsg.Decisions))
					if resumeErr != nil {
						_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(fmt.Sprintf(`{"error":"resume failed: %v"}`, resumeErr))})
						continue
					}
					break
				}
				if wsMsg.Type == "chat" && wsMsg.Text != "" {
					// Client sent a new chat while suspended; cancel old stream and start fresh.
					_ = s.sessionState.DeleteSnapshot(streamCtx, sessionID)
					startStream(wsMsg.Text)
					break
				}
				_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(`{"error":"expected resume or chat"}`)})
			}
			continue
		}

		if _, isEnd := ev.(*event.ReplyEndEvent); isEnd {
			if err := writeV2Event(ws, ev, sessionID, useAGUI, aguiConv); err != nil {
				break
			}
			_ = s.sessionState.DeleteSnapshot(streamCtx, sessionID)
			needsStreamStart = true
			continue
		}

		if err := writeV2Event(ws, ev, sessionID, useAGUI, aguiConv); err != nil {
			break
		}
	}
done:
	if streamCancel != nil {
		streamCancel()
	}
}

func writeV2Event(ws *wsSession, ev event.AgentEvent, sessionID string, useAGUI bool, conv *DefaultAGUIConverter) error {
	data, err := EncodeStreamEvent(ev, AGUIConvertOptions{ThreadID: sessionID}, useAGUI, conv)
	if err != nil {
		return err
	}
	return ws.writeJSON(json.RawMessage(data))
}
