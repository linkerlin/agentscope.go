package sessionapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

// This file is the independent-test proof for the registration-functionized
// shape: the Handlers type is exercised through fake implementations of its
// own narrow interfaces — no gateway root, no *Server involved.

// --- fakes ---

type fakeSessions struct {
	runErr   error
	ran      []string
	steered  string
	steerErr error
}

func (f *fakeSessions) Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	if f.runErr != nil {
		return nil, f.runErr
	}
	f.ran = append(f.ran, sessionID)
	ch := make(chan event.AgentEvent, 1)
	ch <- event.NewReplyStart("r1", "fake")
	close(ch)
	return ch, nil
}

func (f *fakeSessions) Subscribe(sessionID string) <-chan event.AgentEvent {
	ch := make(chan event.AgentEvent)
	close(ch)
	return ch
}

func (f *fakeSessions) Terminate(sessionID string) bool { return false }

func (f *fakeSessions) ClearCompleted(sessionID string) {}

func (f *fakeSessions) Steer(sessionID, text string) error {
	f.steered = text
	return f.steerErr
}

func (f *fakeSessions) IsActive(sessionID string) bool { return false }

func (f *fakeSessions) Suspended(sessionID string) bool { return false }

func (f *fakeSessions) HasCompleted(sessionID string) bool { return false }

type fakeCoordinator struct {
	busy   bool // Run returns ErrSessionBusy
	status SessionStatus
}

func (f *fakeCoordinator) Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	if f.busy {
		return nil, ErrSessionBusy
	}
	ch := make(chan event.AgentEvent, 1)
	ch <- event.NewReplyStart("r1", "fake")
	close(ch)
	return ch, nil
}

func (f *fakeCoordinator) Cancel(ctx context.Context, sessionID string) error { return nil }

func (f *fakeCoordinator) Status(ctx context.Context, sessionID string) SessionStatus {
	return f.status
}

type fakeAgent struct{ name string }

func (f *fakeAgent) Name() string { return f.name }
func (f *fakeAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (f *fakeAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}
func (f *fakeAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (f *fakeAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	close(ch)
	return ch, nil
}
func (f *fakeAgent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (f *fakeAgent) LoadState(*agent.AgentState) error     { return nil }
func (f *fakeAgent) InjectEvent(context.Context, event.AgentEvent) error {
	return nil
}

// newFakeHandlers wires Handlers purely from fakes.
func newFakeHandlers(s *fakeSessions, c Coordinator) *Handlers {
	ag := &fakeAgent{name: "fake"}
	return NewHandlers(Deps{
		Sessions:     s,
		Coordinator:  c,
		Agent:        ag,
		ResolveAgent: func(r *http.Request, agentID, sessionID string) (agent.Agent, error) { return ag, nil },
		EnrichCtx:    func(ctx context.Context, agentID, sessionID string) context.Context { return ctx },
	})
}

// TestHandlers_BusyMapsToConflict proves the 409 contract at the unit level:
// a coordinator returning ErrSessionBusy makes POST answer 409.
func TestHandlers_BusyMapsToConflict(t *testing.T) {
	h := newFakeHandlers(&fakeSessions{}, &fakeCoordinator{busy: true})
	mux := http.NewServeMux()
	h.RegisterV2(mux, nil)

	req := httptest.NewRequest(http.MethodPost, "/v2/chat", strings.NewReader(`{"text":"hi","session_id":"s1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestHandlers_StatusFromCoordinator proves the status endpoint reads the
// coordinator's verdict without any gateway involvement.
func TestHandlers_StatusFromCoordinator(t *testing.T) {
	h := newFakeHandlers(&fakeSessions{}, &fakeCoordinator{status: StatusParked})
	mux := http.NewServeMux()
	h.RegisterV2(mux, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s1/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "s1", resp.SessionID)
	assert.Equal(t, "parked", resp.Status)
}

// TestHandlers_SteerAndErrorMapping exercises steer success and its 409
// mapping through the fake Sessions.
func TestHandlers_SteerAndErrorMapping(t *testing.T) {
	s := &fakeSessions{steerErr: errors.New("no active run")}
	h := newFakeHandlers(s, nil)
	mux := http.NewServeMux()
	h.RegisterV2(mux, nil)

	post := func(text string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v2/sessions/s1/steer", strings.NewReader(`{"text":"`+text+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}

	w := post("focus on tests")
	assert.Equal(t, http.StatusConflict, w.Code, "steer error maps to 409")
	assert.Equal(t, "focus on tests", s.steered)

	// A healthy fake steers fine.
	s2 := &fakeSessions{}
	h2 := newFakeHandlers(s2, nil)
	mux2 := http.NewServeMux()
	h2.RegisterV2(mux2, nil)
	req := httptest.NewRequest(http.MethodPost, "/v2/sessions/s1/steer", strings.NewReader(`{"text":"go"}`))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux2.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "go", s2.steered)
}

// fakeState is a SessionState fake whose Resume returns the configured
// error — enough to prove the HTTP mapping of the 23.2 idempotency
// sentinels without any storage.
type fakeState struct{ resumeErr error }

func (f *fakeState) SaveSnapshot(ctx context.Context, sessionID string, v2 agent.V2Agent) error {
	return nil
}
func (f *fakeState) LoadSnapshot(ctx context.Context, sessionID string, v2 agent.V2Agent) (*service.AgentSnapshot, error) {
	return nil, nil
}
func (f *fakeState) HasPendingSnapshot(ctx context.Context, sessionID string) bool { return true }
func (f *fakeState) Resume(ctx context.Context, sessionID string, v2 agent.V2Agent, ev event.AgentEvent) error {
	return f.resumeErr
}
func (f *fakeState) DeleteSnapshot(ctx context.Context, sessionID string) error { return nil }

// TestHandlers_ResumeIdempotencyMapping locks the 23.2 HTTP contract:
// ErrResumeAlreadyExecuting and ErrResumeNotDelivered both map to 409 with
// the persisted state named in the body.
func TestHandlers_ResumeIdempotencyMapping(t *testing.T) {
	post := func(h *Handlers) *httptest.ResponseRecorder {
		mux := http.NewServeMux()
		h.RegisterV2(mux, nil)
		req := httptest.NewRequest(http.MethodPost, "/v2/resume",
			strings.NewReader(`{"session_id":"s1","reply_id":"r1","confirm_id":"c1","decisions":[{"tool_call_id":"tc1","decision":"allow"}]}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}

	ag := &fakeAgent{name: "fake"}
	// Already executing → 409 executing.
	h := NewHandlers(Deps{
		Sessions:     &fakeSessions{},
		Agent:        ag,
		State:        &fakeState{resumeErr: ErrResumeAlreadyExecuting},
		ResolveAgent: func(r *http.Request, agentID, sessionID string) (agent.Agent, error) { return ag, nil },
		EnrichCtx:    func(ctx context.Context, agentID, sessionID string) context.Context { return ctx },
	})
	w := post(h)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "executing")

	// Not delivered (no live waiter on this replica) → 409 pending.
	h2 := NewHandlers(Deps{
		Sessions:     &fakeSessions{},
		Agent:        ag,
		State:        &fakeState{resumeErr: ErrResumeNotDelivered},
		ResolveAgent: func(r *http.Request, agentID, sessionID string) (agent.Agent, error) { return ag, nil },
		EnrichCtx:    func(ctx context.Context, agentID, sessionID string) context.Context { return ctx },
	})
	w2 := post(h2)
	assert.Equal(t, http.StatusConflict, w2.Code)
	assert.Contains(t, w2.Body.String(), "pending")
}

// TestHandlers_ResumeNotDeliveredNotifiesWorker: when a resume command is
// persisted without a local waiter, the wakeup notification fires so a
// worker replica consumes the command (18.5).
func TestHandlers_ResumeNotDeliveredNotifiesWorker(t *testing.T) {
	var notified []string
	var mu sync.Mutex
	ag := &fakeAgent{name: "fake"}
	h := NewHandlers(Deps{
		Sessions: &fakeSessions{},
		Agent:    ag,
		State:    &fakeState{resumeErr: ErrResumeNotDelivered},
		ResolveAgent: func(r *http.Request, agentID, sessionID string) (agent.Agent, error) {
			return ag, nil
		},
		EnrichCtx: func(ctx context.Context, agentID, sessionID string) context.Context { return ctx },
		OnResumeNotified: func(sessionID string) {
			mu.Lock()
			defer mu.Unlock()
			notified = append(notified, sessionID)
		},
	})
	mux := http.NewServeMux()
	h.RegisterV2(mux, nil)
	req := httptest.NewRequest(http.MethodPost, "/v2/resume",
		strings.NewReader(`{"session_id":"s9","reply_id":"r1","confirm_id":"c1"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	mu.Lock()
	defer mu.Unlock()
	if len(notified) != 1 || notified[0] != "s9" {
		t.Fatalf("wakeup notification missing or wrong: %v", notified)
	}
}
