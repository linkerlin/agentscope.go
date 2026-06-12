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
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
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
// When an authenticator is configured, V2 routes require authentication.
type Server struct {
	agent         agent.Agent
	mux           *http.ServeMux
	authenticator service.Authenticator
	storage       service.Storage
	sessionState  *SessionStateManager
	cipher        *service.Cipher
	otelHandler   http.Handler
	sessions      map[string]*wsSession
	rooms         map[string]map[string]*wsSession
	mu            sync.RWMutex

	// Multi-agent & session management (V2 service layer)
	registry            *AgentRegistry
	sessionMgr          *SessionManager
	backgroundTaskMgr   *BackgroundTaskManager
	modelCardsDir       string
	toolOffload         *ToolOffloadManager
	workspaceMgr        *WorkspaceManager
	embeddingModel      model.EmbeddingModel
	audioModel          model.AudioModel
	sessionAgentBuilder SessionAgentBuilder

	// defaultSessionDeps holds auto-assembled defaults for per-session agents
	// (populated by NewApp when AutoStandardTools etc. are enabled).
	defaultSessionDeps SessionAgentDeps
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
	s.mux.HandleFunc("/health", s.handleHealth)
	return s
}

// WithAuthenticator configures the gateway to require authentication on protected routes.
func (s *Server) WithAuthenticator(auth service.Authenticator) *Server {
	s.authenticator = auth
	return s
}

// WithStorage attaches a service storage for management endpoints and
// automatically creates a SessionStateManager if storage is non-nil.
func (s *Server) WithStorage(st service.Storage) *Server {
	s.storage = st
	if st != nil {
		s.sessionState = NewSessionStateManager(st)
	}
	return s
}

// WithSessionStateManager explicitly sets the session state manager.
// This overrides any manager created by WithStorage.
func (s *Server) WithSessionStateManager(m *SessionStateManager) *Server {
	s.sessionState = m
	return s
}

// WithRegistry attaches an AgentRegistry for multi-agent support.
func (s *Server) WithRegistry(r *AgentRegistry) *Server {
	s.registry = r
	return s
}

// WithSessionManager attaches a SessionManager for per-session
// serialisation, fan-out and replay.
func (s *Server) WithSessionManager(m *SessionManager) *Server {
	s.sessionMgr = m
	return s
}

// WithToolOffloadManager attaches a tool offload manager for background tool hints.
func (s *Server) WithToolOffloadManager(m *ToolOffloadManager) *Server {
	s.toolOffload = m
	return s
}

func (s *Server) toolOffloadMgr() *ToolOffloadManager {
	if s.toolOffload != nil {
		return s.toolOffload
	}
	if s.backgroundTaskMgr != nil {
		return s.backgroundTaskMgr.ToolOffload()
	}
	return nil
}

// WithBackgroundTaskManager attaches a BackgroundTaskManager for cron-based
// agent execution.
func (s *Server) WithBackgroundTaskManager(m *BackgroundTaskManager) *Server {
	s.backgroundTaskMgr = m
	return s
}

// Start starts background components such as the schedule cron (BackgroundTaskManager).
// Call this after wiring routes but before serving traffic. It is safe to call multiple times.
func (s *Server) Start() {
	if s.backgroundTaskMgr != nil {
		s.backgroundTaskMgr.Start()
	}
}

// Close stops background components (schedules, etc.). Call on shutdown.
func (s *Server) Close() error {
	if s.backgroundTaskMgr != nil {
		s.backgroundTaskMgr.Stop()
	}
	return nil
}

// withDefaultSessionDeps stores defaults that will be merged when building
// per-session agents (used by auto-assembly in NewApp).
func (s *Server) withDefaultSessionDeps(deps SessionAgentDeps) *Server {
	s.defaultSessionDeps = deps
	return s
}

// DefaultSessionDeps returns the currently configured default deps for session agents.
func (s *Server) DefaultSessionDeps() SessionAgentDeps {
	return s.defaultSessionDeps
}

// WithCipher attaches an AES-GCM cipher for credential encryption.
func (s *Server) WithCipher(c *service.Cipher) *Server {
	s.cipher = c
	return s
}

// requireAuth wraps a handler with authentication if an authenticator is configured.
func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	if s.authenticator == nil {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.authenticator.Authenticate(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		h(w, r.WithContext(ctx))
	}
}

// RegisterV2Routes adds the V2 event-stream endpoints (SSE + WS) and the
// resume endpoint for suspend-resume workflows to the gateway server.
// These routes are protected if an authenticator is configured.
func (s *Server) RegisterV2Routes() {
	s.mux.HandleFunc("/v2/chat", s.requireAuth(s.handleV2Chat))
	s.mux.HandleFunc("/v2/chat/stream", s.requireAuth(s.handleV2ChatStream))
	s.mux.HandleFunc("/v2/chat/ws", s.requireAuth(s.handleChatWSV2))
	s.mux.HandleFunc("/v2/resume", s.requireAuth(s.handleV2Resume))
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Inject X-Request-ID for observability.
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = generateID("req")
	}
	w.Header().Set("X-Request-ID", reqID)

	if s.otelHandler != nil {
		s.otelHandler.ServeHTTP(w, r)
		return
	}
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

// resolveAgent returns the agent to use for a request.
// If the request specifies an "agent_id" query parameter or JSON field,
// the registry is consulted (with fallback to the default agent).
func (s *Server) resolveAgent(r *http.Request, agentID string) (agent.Agent, error) {
	if agentID == "" {
		agentID = r.URL.Query().Get("agent_id")
	}
	if agentID == "" {
		return s.agent, nil
	}
	if s.registry == nil {
		return nil, fmt.Errorf("agent_id specified but no registry configured")
	}
	return s.registry.Get(r.Context(), agentID)
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

// handleHealth returns a JSON health status for the gateway.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"status":    "healthy",
		"version":   "2.0.0-rc.1",
		"uptime_ms": time.Since(time.Time{}).Milliseconds(), // simplified
	}
	if s.storage != nil {
		status["storage"] = "configured"
	}
	if s.authenticator != nil {
		status["auth"] = "enabled"
	}
	if s.sessionMgr != nil {
		status["active_sessions"] = s.sessionMgr.ActiveCount()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
