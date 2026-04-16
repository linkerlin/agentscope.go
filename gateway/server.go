package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// chatRequest is the expected JSON body for /chat and /chat/stream.
type chatRequest struct {
	Text string `json:"text"`
}

// streamEvent is a single SSE event sent to the client.
type streamEvent struct {
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
}

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

// Server exposes an agent over HTTP with REST, SSE, and WebSocket endpoints.
// It also supports session-based connection tracking and room broadcasting.
type Server struct {
	agent    agent.Agent
	mux      *http.ServeMux
	sessions map[string]*wsSession
	rooms    map[string]map[string]*wsSession
	mu       sync.RWMutex
}

// NewServer creates a gateway HTTP server for the given agent.
func NewServer(a agent.Agent) *Server {
	s := &Server{
		agent:    a,
		mux:      http.NewServeMux(),
		sessions: make(map[string]*wsSession),
		rooms:    make(map[string]map[string]*wsSession),
	}
	s.mux.HandleFunc("/chat", s.handleChat)
	s.mux.HandleFunc("/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("/chat/ws", s.handleChatWS)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerSession adds a WebSocket session to the server.
func (s *Server) registerSession(ws *wsSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[ws.id] = ws
	if ws.room != "" {
		if s.rooms[ws.room] == nil {
			s.rooms[ws.room] = make(map[string]*wsSession)
		}
		s.rooms[ws.room][ws.id] = ws
	}
}

// unregisterSession removes a WebSocket session.
func (s *Server) unregisterSession(ws *wsSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, ws.id)
	if ws.room != "" && s.rooms[ws.room] != nil {
		delete(s.rooms[ws.room], ws.id)
		if len(s.rooms[ws.room]) == 0 {
			delete(s.rooms, ws.room)
		}
	}
}

// BroadcastToRoom sends a JSON message to all sessions in a room.
func (s *Server) BroadcastToRoom(room string, v interface{}) {
	s.mu.RLock()
	members := make(map[string]*wsSession, len(s.rooms[room]))
	for k, v := range s.rooms[room] {
		members[k] = v
	}
	s.mu.RUnlock()
	for _, sess := range members {
		_ = sess.writeJSON(v)
	}
}

// SessionCount returns the number of active WebSocket sessions.
func (s *Server) SessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := parseChatRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
	resp, err := s.agent.Call(r.Context(), msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"role":    "assistant",
		"content": resp.GetTextContent(),
	})
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := parseChatRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent(req.Text).Build()
	ch, err := s.agent.CallStream(r.Context(), msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent stream error: %v", err), http.StatusInternalServerError)
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

	for chunk := range ch {
		if chunk == nil {
			continue
		}
		ev := streamEvent{Delta: chunk.GetTextContent()}
		data, _ := json.Marshal(ev)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	// Final done event
	data, _ := json.Marshal(streamEvent{Done: true})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func parseChatRequest(body io.ReadCloser) (*chatRequest, error) {
	defer body.Close()
	var req chatRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return nil, err
	}
	if req.Text == "" {
		return nil, errors.New("text is required")
	}
	return &req, nil
}
