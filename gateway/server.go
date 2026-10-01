package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/gateway/sessionapi"

	agentscope "github.com/linkerlin/agentscope.go"
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/evolver"
	"github.com/linkerlin/agentscope.go/hub"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/service/access"
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

// Server exposes an agent over HTTP with REST, SSE, and WebSocket endpoints.
// It also supports session-based connection tracking and room broadcasting
// (via the sessionapi handlers). When an authenticator is configured, V2
// routes require authentication.
type Server struct {
	agent         agent.Agent
	mux           *http.ServeMux
	authenticator service.Authenticator
	storage       service.Storage
	sessionState  *SessionStateManager
	cipher        *service.Cipher
	otelHandler   http.Handler
	mu            sync.RWMutex

	// Auth wiring (22.1): jwtAuth is remembered from NewApp so the auth
	// routes can register without re-passing it; production switches the
	// fail-closed defaults (credentials require a cipher, no identity source
	// means a rejecting authenticator instead of anonymous routes).
	jwtAuth              *service.JWTAuthenticator
	production           bool
	authRoutesRegistered bool

	// Cross-tenant resource sharing (22.2): nil = deny all cross-user access
	// (DenyAllPolicy semantics); owners always keep full access to their own
	// resources.
	accessPolicy access.Policy

	// Multi-agent & session management (V2 service layer)
	registry            *AgentRegistry
	sessionMgr          *SessionManager
	sessionCoord        *SessionCoordinator
	backgroundTaskMgr   *BackgroundTaskManager
	modelCardsDir       string
	toolOffload         *ToolOffloadManager
	workspaceMgr        *WorkspaceManager
	embeddingModel      model.EmbeddingModel
	audioModel          model.AudioModel
	sessionAgentBuilder SessionAgentBuilder
	// messageBus enables cross-process coordination (distributed cancel /
	// wake-up / tool-offload-complete events). nil = single-process (no bus).
	messageBus messagebus.Bus
	// wakeupDispatcher drains team inboxes and re-runs idle worker sessions.
	// Auto-started in Start() when the bus implements TeamBus.
	wakeupDispatcher *WakeupDispatcher

	// worker roles (18.5): which long-running loops this process owns.
	// workerRoles nil = all (single-process default); an explicitly-set empty
	// slice = none (dedicated API replica). When the bus carries fencing
	// leases the roles are additionally guarded by a Worker (competitive
	// acquire, heartbeat, takeover).
	workerRolesExplicit bool
	workerRoles         []WorkerRole
	worker              *Worker
	// directRunners holds the role pairs started by the non-Worker path
	// (no fencing leases on the bus); Close stops them.
	directRunners []RoleRunner

	// rateLimiter guards the public write endpoints (18.11): login and
	// register. Auto-selected at Start from the bus's CoordCounter
	// capability; WithRateLimiter overrides, WithRateLimitDisabled opts out.
	rateLimiter       RateLimiter
	rateLimitDisabled bool

	// dingtalkAgentResolver resolves the agent for a HITL card callback
	// (18.3); nil = buildSessionAgentFromStorage. Injectable for custom
	// session-agent resolution.
	dingtalkAgentResolver func(ctx context.Context, agentID, sessionID string) (agent.Agent, error)

	// session HTTP face (16.2): lazily-built sessionapi handlers bridging
	// the Server wiring; see sessionapi_compat.go.
	sessionAPIBuild    sync.Once
	sessionAPIHandlers *sessionapi.Handlers

	// kbService powers the knowledge-base HTTP API (CRUD + upload + search).
	// nil disables KB routes. Attach via WithKBService.
	kbService *KBService

	// auditLogger records authenticated requests (who/method/path/status).
	// nil disables auditing. Attach via WithAuditLogger.
	auditLogger service.AuditLogger

	// channel subsystem (multi-platform messaging). nil = disabled.
	channelRegistry   *channel.Registry
	channelGateway    *channel.Gateway
	channelDispatcher *channel.Dispatcher
	channelsStarted   bool

	// hubs are marketplace registries for MCP/skill install. empty = disabled.
	hubs []hub.Hub

	// defaultSessionDeps holds auto-assembled defaults for per-session agents
	// (populated by NewApp when AutoStandardTools etc. are enabled).
	defaultSessionDeps SessionAgentDeps

	// controlPlane is the long-running-agent governance kernel (LoopX-style).
	// nil = disabled (default); attach via WithControlPlane to enable the
	// /api/v1/controlplane/* routes and the ControlPlaneMiddleware path.
	controlPlane *controlplane.Kernel

	// evolver powers the governance->evolution closed loop: when autoSolidify
	// is enabled, goal completion auto-solidifies into evolver capsules.
	evolver      evolver.Evolver
	autoSolidify bool
}

// NewServer creates a gateway HTTP server for the given agent.
func NewServer(a agent.Agent) *Server {
	s := &Server{
		agent: a,
		mux:   http.NewServeMux(),
	}
	s.mux.HandleFunc("/chat", s.handleChat)
	s.mux.HandleFunc("/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("/chat/ws", s.legacyWSHandler)
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

// WithMessageBus attaches a pub/sub bus for cross-process coordination
// (distributed cancel / wake-up / tool-offload events). Optional; nil keeps the
// server single-process. Aligns with Python agentscope's message bus (#1849).
func (s *Server) WithMessageBus(b messagebus.Bus) *Server {
	s.messageBus = b
	return s
}

// MessageBus returns the attached bus (may be nil in single-process deployments).
func (s *Server) MessageBus() messagebus.Bus { return s.messageBus }

// WithSessionManager attaches a SessionManager for per-session
// serialisation, fan-out and replay.
func (s *Server) WithSessionManager(m *SessionManager) *Server {
	s.sessionMgr = m
	return s
}

// WithSessionCoordinator attaches the cross-replica session coordinator
// (18.1/18.2). When set, HTTP session runs go through it (run lock, event
// log, cross-process cancel) and the status endpoint reads its data plane.
// Ordering: call BEFORE RegisterV2Routes — the sessionapi handlers snapshot
// the server wiring at registration time (same requirement as the
// authenticator).
func (s *Server) WithSessionCoordinator(c *SessionCoordinator) *Server {
	s.sessionCoord = c
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
//
// 18.5: the long-running loops (wakeup drain, cron scheduler, channel
// listeners) are selected by WithWorkerRoles. When the bus carries fencing
// leases (CoordLease) they are additionally guarded by a Worker — role
// leases with heartbeat and takeover, so a dedicated worker process and API
// replicas (or two workers) can coexist without double consumption. Without
// leases the selected roles start in-process directly.
func (s *Server) Start() {
	// Rate limiter auto-selection (18.11): shared CoordCounter when the bus
	// provides one (exact cross-replica budget), else process-local buckets.
	// Explicit WithRateLimiter / WithRateLimitDisabled win over auto.
	if s.rateLimiter == nil && !s.rateLimitDisabled {
		if cc := messagebus.AsCoordCounter(s.messageBus); cc != nil {
			s.rateLimiter = NewCoordRateLimiter(cc, DefaultRateLimit, DefaultRateWindow)
		} else {
			s.rateLimiter = NewLocalRateLimiter(DefaultRateLimit, DefaultRateWindow)
		}
	}

	roles := s.workerRoles
	if !s.workerRolesExplicit {
		roles = AllWorkerRoles
	}
	// Build the per-role start/stop pairs first.
	runners := s.workerRunners(roles)

	if lease := messagebus.AsCoordLease(s.messageBus); lease != nil && len(runners) > 0 {
		w := NewWorker(lease, generateID("worker"), runners)
		s.worker = w
		w.Start(context.Background())
		return
	}
	// No fencing leases (or no roles): start the selected roles directly —
	// the loops run in-process exclusively; Close stops them via the
	// recorded runners.
	s.directRunners = runners
	for _, r := range runners {
		_ = r.Start()
	}
}

// workerRunners binds the selected roles to idempotent start/stop pairs.
// Components are held by the closures themselves (the Worker loop calls
// Start/Stop serially on its own goroutine); Server fields like
// wakeupDispatcher stay owned by the non-Worker direct path, so no shared
// mutable state crosses goroutines.
func (s *Server) workerRunners(roles []WorkerRole) []RoleRunner {
	wanted := make(map[WorkerRole]bool, len(roles))
	for _, r := range roles {
		wanted[r] = true
	}
	var runners []RoleRunner
	if wanted[RoleWakeup] && s.supportsWakeup() {
		var disp *WakeupDispatcher
		runners = append(runners, RoleRunner{
			Role: RoleWakeup,
			Start: func() error {
				d := s.buildWakeupDispatcher()
				if d == nil {
					return fmt.Errorf("wakeup dispatcher unavailable")
				}
				if err := d.Start(context.Background()); err != nil {
					return err
				}
				disp = d
				return nil
			},
			Stop: func() {
				if disp != nil {
					disp.Stop()
					disp = nil
				}
			},
		})
	}
	if wanted[RoleSchedule] && s.backgroundTaskMgr != nil {
		runners = append(runners, RoleRunner{
			Role: RoleSchedule,
			Start: func() error {
				s.backgroundTaskMgr.Start()
				return nil
			},
			Stop: func() { s.backgroundTaskMgr.Stop() },
		})
	}
	if wanted[RoleChannel] && s.channelDispatcher != nil {
		runners = append(runners, RoleRunner{
			Role: RoleChannel,
			Start: func() error {
				go func() { _ = s.channelDispatcher.StartAll(context.Background()) }()
				return nil
			},
			Stop: func() { s.channelDispatcher.Close() },
		})
	}
	return runners
}

// supportsWakeup reports whether the wakeup loop can run on this server.
func (s *Server) supportsWakeup() bool {
	return messagebus.AsTeamBus(s.messageBus) != nil && s.sessionMgr != nil && s.storage != nil
}

// WithWorkerRoles selects which long-running loops this process owns (18.5):
// no arguments = none (dedicated API replica); omit the call entirely = all
// roles (single-process default).
func (s *Server) WithWorkerRoles(roles ...WorkerRole) *Server {
	s.workerRolesExplicit = true
	s.workerRoles = roles
	return s
}

// WorkerRoles reports the selected worker roles (nil before Start when the
// default set applies).
func (s *Server) WorkerRoles() []WorkerRole { return s.workerRoles }

// CoordinatedRun returns the server's coordinated turn entry — the session
// coordinator when wired, else the in-process session manager. Assemblers
// inject it into ChannelRunner.Run / BackgroundTaskManager.WithSessionRun so
// every turn source (HTTP, channel, cron, wakeup) shares one cross-replica
// single-consumer protocol (18.5). Returns nil when neither is wired.
func (s *Server) CoordinatedRun() sessionRunFunc {
	if s.sessionCoord != nil {
		return s.sessionCoord.Run
	}
	if s.sessionMgr != nil {
		return s.sessionMgr.Run
	}
	return nil
}

// WithRateLimiter overrides the auto-selected rate limiter for the public
// write endpoints (18.11). Call before Start.
func (s *Server) WithRateLimiter(l RateLimiter) *Server {
	s.rateLimiter = l
	return s
}

// WithRateLimitDisabled opts out of endpoint throttling entirely (tests,
// trusted inner deployments). The default is throttling with the package
// defaults (30 req/min per identity, cross-replica when the bus provides a
// CoordCounter).
func (s *Server) WithRateLimitDisabled() *Server {
	s.rateLimitDisabled = true
	s.rateLimiter = nil
	return s
}

// startWakeupDispatcher launches the team-collaboration wakeup loop outside
// Start's role management (manual/legacy wiring). Idempotent. Start() itself
// routes the wakeup role through workerRunners (18.5); this entry remains
// for callers that assemble the loop by hand.
func (s *Server) startWakeupDispatcher() {
	if s.wakeupDispatcher != nil {
		return
	}
	d := s.buildWakeupDispatcher()
	if d == nil {
		return
	}
	if err := d.Start(context.Background()); err == nil {
		s.wakeupDispatcher = d
	}
}

// buildWakeupDispatcher assembles the wakeup loop when the configured bus
// implements TeamBus and the required managers are present; nil when the
// prerequisites are missing. Turns go through the coordinator when one is
// wired, and persisted resume commands are consumed here — the wakeup loop
// is the worker tier's entry point (18.5).
func (s *Server) buildWakeupDispatcher() *WakeupDispatcher {
	if !s.supportsWakeup() {
		return nil
	}
	tb := messagebus.AsTeamBus(s.messageBus)
	d := NewWakeupDispatcher(tb, s.sessionMgr, s.storage, s.buildSessionAgentFromStorage)
	if s.sessionCoord != nil {
		d.WithRun(s.sessionCoord.Run)
	}
	return d
}

// Close stops background components (worker roles, schedules, etc.). Call on
// shutdown.
func (s *Server) Close() error {
	if s.worker != nil {
		s.worker.Stop()
		s.worker = nil
	}
	for _, r := range s.directRunners {
		r.Stop()
	}
	s.directRunners = nil
	if s.wakeupDispatcher != nil {
		s.wakeupDispatcher.Stop()
	}
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

// WithJWTAuth remembers the JWT authenticator so RegisterAuthRoutes and
// RegisterAppRoutes can be called without re-passing it (22.1). It does not
// enable request authentication by itself — pair with WithAuthenticator or
// let NewApp assemble the chain.
func (s *Server) WithJWTAuth(j *service.JWTAuthenticator) *Server {
	s.jwtAuth = j
	return s
}

// WithProduction switches the server to production semantics (22.1):
// credential writes require a cipher. Pair with NewApp, which also installs
// a rejecting authenticator when production mode has no identity source, so
// business routes fail closed instead of staying anonymous.
func (s *Server) WithProduction(on bool) *Server {
	s.production = on
	return s
}

// WithAccessPolicy attaches the cross-tenant resource-sharing policy (22.2).
// Without one, cross-user access to agent configs is denied (owners keep
// full access to their own resources).
func (s *Server) WithAccessPolicy(p access.Policy) *Server {
	s.accessPolicy = p
	return s
}

// requireAuth wraps a handler with authentication if an authenticator is configured.
// When an audit logger is also set, authenticated requests are recorded automatically.
func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	if s.authenticator == nil {
		return s.auditWrapped(h)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.authenticator.Authenticate(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		s.auditWrapped(h)(w, r.WithContext(ctx))
	}
}

// RegisterV2Routes adds the V2 session endpoints (streamable HTTP + SSE
// alias + WebSocket + resume + steer/interrupt + session status), mounted
// through the sessionapi handlers (16.2). Protected if an authenticator is
// configured. Wire storage / session manager / coordinator before calling.
func (s *Server) RegisterV2Routes() {
	s.sessionAPI().RegisterV2(s.mux, s.requireAuth)
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

// registerSession/unregisterSession and the room broadcast moved into the
// sessionapi handlers; Server.BroadcastToRoom / SessionCount (in
// sessionapi_compat.go) delegate to them.

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
		"version":   agentscope.Version,
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
