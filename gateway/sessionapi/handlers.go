package sessionapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

// Sessions is the in-process session runtime the HTTP face drives. Satisfied
// structurally by the gateway root's *SessionManager.
type Sessions interface {
	Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)
	Subscribe(sessionID string) <-chan event.AgentEvent
	Terminate(sessionID string) bool
	ClearCompleted(sessionID string)
	Steer(sessionID, text string) error
	IsActive(sessionID string) bool
	Suspended(sessionID string) bool
	HasCompleted(sessionID string) bool
}

// Coordinator is the optional cross-replica session coordinator (18.1/18.2).
// Satisfied structurally by the gateway root's *SessionCoordinator.
type Coordinator interface {
	Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)
	Cancel(ctx context.Context, sessionID string) error
	Status(ctx context.Context, sessionID string) SessionStatus
}

// SessionState is the suspend/resume snapshot lifecycle. Satisfied
// structurally by the gateway root's *SessionStateManager; nil disables
// snapshot persistence.
type SessionState interface {
	SaveSnapshot(ctx context.Context, sessionID string, v2 agent.V2Agent) error
	LoadSnapshot(ctx context.Context, sessionID string, v2 agent.V2Agent) (*service.AgentSnapshot, error)
	HasPendingSnapshot(ctx context.Context, sessionID string) bool
	Resume(ctx context.Context, sessionID string, v2 agent.V2Agent, ev event.AgentEvent) error
	DeleteSnapshot(ctx context.Context, sessionID string) error
}

// ErrStorageNotAvailable mirrors the root session-state sentinel so resume
// handlers can map it to 503 without importing the gateway root.
var ErrStorageNotAvailable = errors.New("session_state: storage not available")

// ErrResumeAlreadyExecuting is returned when a resume command for a
// confirm_id was already delivered and is mid-execution (23.2 idempotency):
// the HTTP face maps it to 409 and the command is NOT re-delivered.
var ErrResumeAlreadyExecuting = errors.New("session_state: resume already executing")

// ErrResumeNotDelivered is returned when the resume command is persisted but
// no live waiter for it exists on this replica (the run parked elsewhere, or
// died after delivery): the command stays pending for the holder replica.
// The HTTP face maps it to 409 with the persisted state reported.
var ErrResumeNotDelivered = errors.New("session_state: resume not delivered (no live waiter here)")

// SessionStatus is the observable lifecycle state of a session.
type SessionStatus string

const (
	// StatusRunning: a turn is executing on this or another replica.
	StatusRunning SessionStatus = "running"
	// StatusParked: the session is suspended awaiting a human-in-the-loop
	// decision (tool confirmation or external execution).
	StatusParked SessionStatus = "parked"
	// StatusIdle: the session has history but nothing is running or parked.
	StatusIdle SessionStatus = "idle"
	// StatusUnknown: no record of this session anywhere.
	StatusUnknown SessionStatus = "unknown"
)

// ErrSessionBusy is returned when another replica holds the session's run
// lock; the HTTP face maps it to 409.
var ErrSessionBusy = errors.New("session coordinator: session already running on another replica")

// Deps carries the session HTTP face's complete dependency surface: narrow
// interfaces plus injected functions owned by the gateway root. All function
// fields are required unless noted.
type Deps struct {
	Sessions    Sessions
	Coordinator Coordinator     // optional; nil = single-process semantics
	State       SessionState    // optional; nil = no snapshot persistence
	Storage     service.Storage // optional; nil = no tenancy enforcement
	// Agent is the default agent for endpoints without per-request resolution
	// (legacy WS chat, resume).
	Agent agent.Agent
	// ResolveAgent resolves the acting agent for a request (multi-agent).
	ResolveAgent func(r *http.Request, agentID, sessionID string) (agent.Agent, error)
	// EnrichCtx injects per-session context (workspace tools etc.).
	EnrichCtx func(ctx context.Context, agentID, sessionID string) context.Context
	// OffloadHints prepends pending background-tool notifications to a user
	// message; may return text unchanged.
	OffloadHints func(sessionID, text string) string
	// OnResumeNotified is invoked when a resume command is persisted but this
	// replica holds no live waiter (ErrResumeNotDelivered): the gateway root
	// wires it to the bus's wakeup enqueue so a worker replica picks the
	// command up (18.5). Optional; nil disables the notification.
	OnResumeNotified func(sessionID string)
}

// Handlers is the session HTTP face. Build via NewHandlers and mount with
// RegisterV2 / RegisterLegacyWS.
type Handlers struct {
	d  Deps
	ws *wsRegistry
}

// NewHandlers builds the session HTTP handlers over the given deps.
func NewHandlers(d Deps) *Handlers { return &Handlers{d: d, ws: newWSRegistry()} }

// AuthWrapper wraps a handler with the caller's authentication middleware.
type AuthWrapper func(http.HandlerFunc) http.HandlerFunc

// RegisterV2 mounts the V2 session endpoints: streamable HTTP (/v2/chat),
// its SSE-only legacy alias, the V2 WebSocket, resume, steer, interrupt and
// the session status endpoint.
func (h *Handlers) RegisterV2(mux *http.ServeMux, auth AuthWrapper) {
	wrap := auth
	if wrap == nil {
		wrap = func(hf http.HandlerFunc) http.HandlerFunc { return hf }
	}
	mux.HandleFunc("/v2/chat", wrap(h.handleV2Chat))
	mux.HandleFunc("/v2/chat/stream", wrap(h.handleV2ChatStream))
	mux.HandleFunc("/v2/chat/ws", wrap(h.handleChatWSV2))
	mux.HandleFunc("/v2/resume", wrap(h.handleV2Resume))
	mux.HandleFunc("POST /v2/sessions/{session_id}/steer", wrap(h.handleV2Steer))
	mux.HandleFunc("POST /v2/sessions/{session_id}/interrupt", wrap(h.handleV2Interrupt))
	// Session coordination surface (18.1/18.2): status joins the V2 family.
	mux.HandleFunc("GET /api/v1/sessions/{id}/status", wrap(h.handleSessionStatus))
}

// RegisterLegacyWS mounts the unauthenticated V1 WebSocket endpoint.
func (h *Handlers) RegisterLegacyWS(mux *http.ServeMux) {
	mux.HandleFunc("/chat/ws", h.handleChatWS)
}

// run routes one turn through the coordinator when wired, mirroring the
// pre-extraction Server.runSession.
func (h *Handlers) run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	if h.d.Coordinator != nil {
		return h.d.Coordinator.Run(ctx, sessionID, a, msg)
	}
	return h.d.Sessions.Run(ctx, sessionID, a, msg)
}

// checkSessionAccess verifies that a session belongs to the authenticated
// user when storage is available. With storage configured, an unknown session
// ID is refused (22.2): server-side IDs are persisted at creation, so an
// unknown ID is either forged or deleted, and accepting it would let a
// tenant claim an arbitrary ID. It writes 404 (not 403, to avoid leaking
// session existence) and returns false when access is denied. Without storage
// there is nothing to enforce against and access is allowed.
func (h *Handlers) checkSessionAccess(w http.ResponseWriter, r *http.Request, sessionID string) bool {
	if h.d.Storage == nil || sessionID == "" {
		return true
	}
	se, err := h.d.Storage.GetSession(r.Context(), sessionID)
	if err != nil {
		// Unknown session: refuse instead of treating it as a fresh session.
		http.Error(w, "session not found", http.StatusNotFound)
		return false
	}
	userID := service.UserIDFromContext(r.Context())
	if se.UserID != "" && userID != "" && se.UserID != userID {
		http.Error(w, "session not found", http.StatusNotFound)
		return false
	}
	return true
}

// --- streamable HTTP (/v2/chat POST + GET + DELETE) ---

type chatStreamParams struct {
	sessionID    string
	agentID      string
	text         string
	useAGUI      bool
	strictAccept bool
}

// handleV2Chat is the Streamable HTTP endpoint (POST + GET + DELETE).
func (h *Handlers) handleV2Chat(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleV2ChatPost(w, r, chatStreamParams{strictAccept: true})
	case http.MethodGet:
		h.handleV2ChatGet(w, r)
	case http.MethodDelete:
		h.handleV2ChatDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleV2ChatStream keeps the deprecated SSE-only path for older clients.
func (h *Handlers) handleV2ChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.handleV2ChatPost(w, r, chatStreamParams{strictAccept: false})
}

func (h *Handlers) handleV2ChatPost(w http.ResponseWriter, r *http.Request, opts chatStreamParams) {
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
	if !h.checkSessionAccess(w, r, sessionID) {
		return
	}
	if sessionID == "" {
		// New session: the server mints the ID (22.2) — clients cannot claim
		// arbitrary IDs — and persists ownership when storage is configured.
		minted, err := h.ensureSession(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sessionID = minted
	}
	params := chatStreamParams{
		sessionID: sessionID,
		agentID:   req.AgentID,
		text:      req.Text,
		useAGUI:   useAGUIProtocol(r),
	}

	a, err := h.d.ResolveAgent(r, params.agentID, params.sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v2, ok := a.(agent.V2Agent)
	if !ok {
		http.Error(w, "agent does not support V2 streaming", http.StatusNotImplemented)
		return
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent(h.offloadHinted(params.sessionID, params.text)).Build()
	ch, err := h.startAgentEventStream(r, a, v2, params.agentID, params.sessionID, msg)
	if err != nil {
		// Cross-replica busy (18.1): another replica holds the session run
		// lock — surface as 409 instead of a generic 500.
		if errors.Is(err, ErrSessionBusy) {
			http.Error(w, "session already running on another replica", http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("reply stream error: %v", err), http.StatusInternalServerError)
		return
	}

	if params.sessionID != "" {
		w.Header().Set(HeaderAgentSessionID, params.sessionID)
	}

	if wantsJSONOnly(r) {
		h.writeChatJSON(w, ch, params)
		return
	}

	h.writeChatSSE(w, r, ch, v2, params)
}

func (h *Handlers) handleV2ChatGet(w http.ResponseWriter, r *http.Request) {
	if !acceptsStreamableHTTP(r) && r.Header.Get(headerAccept) != "" {
		http.Error(w, "Accept must include text/event-stream", http.StatusNotAcceptable)
		return
	}

	sessionID := firstNonEmpty(r.URL.Query().Get("session_id"), r.Header.Get(HeaderAgentSessionID))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if h.d.Sessions == nil {
		http.Error(w, "session manager not configured", http.StatusServiceUnavailable)
		return
	}
	if !h.checkSessionAccess(w, r, sessionID) {
		return
	}

	params := chatStreamParams{
		sessionID: sessionID,
		useAGUI:   useAGUIProtocol(r),
	}

	w.Header().Set(HeaderAgentSessionID, sessionID)
	ch := h.d.Sessions.Subscribe(sessionID)

	// Subscribe may return empty closed channel; still emit terminal done.
	h.writeChatSSE(w, r, ch, nil, params)
}

func (h *Handlers) handleV2ChatDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := firstNonEmpty(r.URL.Query().Get("session_id"), r.Header.Get(HeaderAgentSessionID))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if h.d.Sessions == nil {
		http.Error(w, "session manager not configured", http.StatusServiceUnavailable)
		return
	}
	if !h.checkSessionAccess(w, r, sessionID) {
		return
	}

	terminated := h.d.Sessions.Terminate(sessionID)
	if !terminated && h.d.Coordinator != nil {
		// Cross-process cancel (18.1): the run may live on another replica.
		// Publish the cancel request; the owning replica terminates the run
		// and performs its own cleanup, so no local state is touched here.
		if cerr := h.d.Coordinator.Cancel(r.Context(), sessionID); cerr == nil {
			w.Header().Set(HeaderAgentSessionID, sessionID)
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}
	if !terminated {
		http.Error(w, "no active run for session", http.StatusNotFound)
		return
	}

	h.d.Sessions.ClearCompleted(sessionID)
	if h.d.State != nil {
		_ = h.d.State.DeleteSnapshot(r.Context(), sessionID)
	}

	w.Header().Set(HeaderAgentSessionID, sessionID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) startAgentEventStream(
	r *http.Request,
	a agent.Agent,
	v2 agent.V2Agent,
	agentID, sessionID string,
	msg *message.Msg,
) (<-chan event.AgentEvent, error) {
	ctx := h.d.EnrichCtx(r.Context(), agentID, sessionID)
	if h.d.Sessions != nil && sessionID != "" {
		return h.run(ctx, sessionID, a, msg)
	}
	return v2.ReplyStream(ctx, msg)
}

func (h *Handlers) writeChatSSE(
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

		if v2 != nil && h.d.State != nil {
			if _, suspended := ev.(*event.RequireUserConfirmEvent); suspended && params.sessionID != "" {
				if err := h.d.State.SaveSnapshot(r.Context(), params.sessionID, v2); err != nil {
					errEv := event.NewError(ev.ReplyID(), fmt.Errorf("save snapshot failed: %w", err))
					_ = sendEvent(errEv)
				}
			}
		}

		if !sendEvent(ev) {
			break
		}

		if params.sessionID != "" && h.d.State != nil {
			if _, isEnd := ev.(*event.ReplyEndEvent); isEnd {
				_ = h.d.State.DeleteSnapshot(r.Context(), params.sessionID)
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

func (h *Handlers) writeChatJSON(w http.ResponseWriter, ch <-chan event.AgentEvent, params chatStreamParams) {
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

// offloadHinted applies the injected offload-hint hook (no-op when unset).
func (h *Handlers) offloadHinted(sessionID, text string) string {
	if h.d.OffloadHints == nil {
		return text
	}
	return h.d.OffloadHints(sessionID, text)
}

// --- steer / interrupt ---

type steerRequest struct {
	Text string `json:"text"`
}

// handleV2Steer injects a user message into an active run (mid-turn steering).
// POST /v2/sessions/{session_id}/steer
func (h *Handlers) handleV2Steer(w http.ResponseWriter, r *http.Request) {
	if h.d.Sessions == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session manager not configured"})
		return
	}
	sessionID := r.PathValue("session_id")
	var req steerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "text is required"})
		return
	}
	if err := h.d.Sessions.Steer(sessionID, req.Text); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleV2Interrupt terminates the active run for a session (agent Interrupt
// + context cancel). POST /v2/sessions/{session_id}/interrupt
func (h *Handlers) handleV2Interrupt(w http.ResponseWriter, r *http.Request) {
	if h.d.Sessions == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session manager not configured"})
		return
	}
	sessionID := r.PathValue("session_id")
	if !h.d.Sessions.Terminate(sessionID) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no active run for session " + sessionID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- session status (18.2) ---

// handleSessionStatus resolves running / parked / idle / unknown from the
// coordinator's data plane, degrading to local-only resolution without one.
func (h *Handlers) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.checkSessionAccess(w, r, id) {
		return
	}
	status := StatusUnknown
	switch {
	case h.d.Coordinator != nil:
		status = h.d.Coordinator.Status(r.Context(), id)
	case h.d.Sessions != nil:
		// No coordinator wired: degrade to local-only resolution.
		switch {
		case h.d.Sessions.IsActive(id):
			if h.d.Sessions.Suspended(id) {
				status = StatusParked
			} else {
				status = StatusRunning
			}
		case h.d.Sessions.HasCompleted(id):
			status = StatusIdle
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": id,
		"status":     status,
	})
}
