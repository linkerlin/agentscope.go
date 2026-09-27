package sessionapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
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

// wsSession wraps a WebSocket connection with safe concurrent writes.
type wsSession struct {
	id       string
	room     string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	lastPing time.Time
}

func (s *wsSession) writeJSON(v interface{}) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(v)
}

func (s *wsSession) writeControl(messageType int, data []byte, deadline time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteControl(messageType, data, deadline)
}

func (s *wsSession) close() {
	s.writeMu.Lock()
	_ = s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	s.writeMu.Unlock()
	s.conn.Close()
}

// wsRegistry tracks live WebSocket connections by session and room.
type wsRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*wsSession
	rooms    map[string]map[string]*wsSession
}

func newWSRegistry() *wsRegistry {
	return &wsRegistry{
		sessions: make(map[string]*wsSession),
		rooms:    make(map[string]map[string]*wsSession),
	}
}

func (reg *wsRegistry) register(ws *wsSession) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.sessions[ws.id] = ws
	if ws.room != "" {
		if reg.rooms[ws.room] == nil {
			reg.rooms[ws.room] = make(map[string]*wsSession)
		}
		reg.rooms[ws.room][ws.id] = ws
	}
}

func (reg *wsRegistry) unregister(ws *wsSession) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	delete(reg.sessions, ws.id)
	if ws.room != "" {
		if m, ok := reg.rooms[ws.room]; ok {
			delete(m, ws.id)
			if len(m) == 0 {
				delete(reg.rooms, ws.room)
			}
		}
	}
}

// HandleChatWS is the exported legacy WebSocket entry: the gateway root
// mounts /chat/ws at construction time, before session wiring completes, so
// it delegates here through a lazily-built Handlers.
func (h *Handlers) HandleChatWS(w http.ResponseWriter, r *http.Request) {
	h.handleChatWS(w, r)
}

// BroadcastToRoom sends a JSON message to all WebSocket sessions in a room.
func (h *Handlers) BroadcastToRoom(room string, v interface{}) {
	h.ws.mu.RLock()
	members := make(map[string]*wsSession, len(h.ws.rooms[room]))
	for k, v := range h.ws.rooms[room] {
		members[k] = v
	}
	h.ws.mu.RUnlock()
	for _, sess := range members {
		_ = sess.writeJSON(v)
	}
}

// SessionCount returns the number of active WebSocket sessions.
func (h *Handlers) SessionCount() int {
	h.ws.mu.RLock()
	defer h.ws.mu.RUnlock()
	return len(h.ws.sessions)
}

func (h *Handlers) handleChatWS(w http.ResponseWriter, r *http.Request) {
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
	h.ws.register(ws)
	defer func() {
		h.ws.unregister(ws)
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
		ch, err := h.d.Agent.CallStream(r.Context(), msg)
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
	Type      string                  `json:"type"`
	Text      string                  `json:"text,omitempty"`
	ConfirmID string                  `json:"confirm_id,omitempty"`
	ReplyID   string                  `json:"reply_id,omitempty"`
	Decisions []event.ConfirmDecision `json:"decisions,omitempty"`
}

// handleChatWSV2 serves the V2 WebSocket endpoint that streams AgentEvents.
// It supports suspend-resume: when a RequireUserConfirmEvent is emitted,
// the stream pauses and the AgentState is saved to Storage (if configured).
// The client must send a "resume" message with decisions to continue.
func (h *Handlers) handleChatWSV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	sessionID := r.URL.Query().Get("session")
	a, err := h.d.ResolveAgent(r, agentID, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v2, ok := a.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	if !h.checkSessionAccess(w, r, sessionID) {
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	room := r.URL.Query().Get("room")

	ws := &wsSession{
		id:       sessionID,
		room:     room,
		conn:     conn,
		lastPing: time.Now(),
	}
	h.ws.register(ws)
	defer func() {
		h.ws.unregister(ws)
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
		streamCtx = h.d.EnrichCtx(streamCtx, agentID, sessionID)
		msg := message.NewMsg().Role(message.RoleUser).TextContent(h.offloadHinted(sessionID, text)).Build()

		var evCh <-chan event.AgentEvent
		var err error
		if h.d.Sessions != nil && sessionID != "" {
			evCh, err = h.run(streamCtx, sessionID, a, msg)
		} else {
			evCh, err = v2.ReplyStream(streamCtx, msg)
		}
		if err != nil {
			// WS frames have no status code: a cross-replica busy is flagged
			// in the error payload so clients can react (retry/subscribe).
			payload := fmt.Sprintf(`{"error":"%v"}`, err)
			if errors.Is(err, ErrSessionBusy) {
				payload = fmt.Sprintf(`{"error":"%v","busy":true}`, err)
			}
			_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(payload)})
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
			if sessionID != "" && h.d.State != nil && h.d.State.HasPendingSnapshot(r.Context(), sessionID) {
				if _, err := h.d.State.LoadSnapshot(r.Context(), sessionID, v2); err == nil {
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
			if h.d.State != nil {
				if err := h.d.State.SaveSnapshot(streamCtx, sessionID, v2); err != nil {
					_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(fmt.Sprintf(`{"error":"save snapshot failed: %v"}`, err))})
				}
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
					// nil State (no storage) keeps the old nil-receiver
					// semantics: in-memory resume via InjectEvent.
					var resumeErr error
					if h.d.State != nil {
						resumeErr = h.d.State.Resume(streamCtx, sessionID, v2,
							event.NewUserConfirmResult(wsMsg.ReplyID, wsMsg.ConfirmID, wsMsg.Decisions))
					} else {
						resumeErr = v2.InjectEvent(streamCtx, event.NewUserConfirmResult(wsMsg.ReplyID, wsMsg.ConfirmID, wsMsg.Decisions))
					}
					if resumeErr != nil {
						_ = ws.writeJSON(v2Event{EventType: "error", Payload: []byte(fmt.Sprintf(`{"error":"resume failed: %v"}`, resumeErr))})
						continue
					}
					break
				}
				if wsMsg.Type == "chat" && wsMsg.Text != "" {
					// Client sent a new chat while suspended; cancel old stream and start fresh.
					if h.d.State != nil {
						_ = h.d.State.DeleteSnapshot(streamCtx, sessionID)
					}
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
			if h.d.State != nil {
				_ = h.d.State.DeleteSnapshot(streamCtx, sessionID)
			}
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
