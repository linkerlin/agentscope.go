package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/messagebus"
)

// coordServer builds a Server wired with a coordinator over the shared bus
// and registers the /v2/chat routes, so the HTTP surface can be exercised.
// Wiring order matters: the coordinator must be attached BEFORE
// RegisterV2Routes — the sessionapi handlers snapshot the server wiring at
// registration time (same ordering requirement as the authenticator).
func coordServer(t *testing.T, bus messagebus.Bus) *Server {
	t.Helper()
	srv := NewServer(makeMockAgent(nil, 0))
	srv.WithSessionManager(NewSessionManager())
	coord := NewSessionCoordinator(srv.sessionMgr).WithBus(bus).WithLockAcquireTimeout(50 * time.Millisecond)
	srv.WithSessionCoordinator(coord)
	srv.RegisterV2Routes()
	srv.RegisterServiceRoutes()
	return srv
}

func postChat(t *testing.T, srv *Server, sessionID, text string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"text": "` + text + `", "session_id": "` + sessionID + `"}`)
	req := httptest.NewRequest("POST", "/v2/chat", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func deleteChat(t *testing.T, srv *Server, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("DELETE", "/v2/chat?session_id="+sessionID, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// TestSessionBusyHTTP409 verifies the acceptance clause: when another replica
// holds the session run lock, POST /v2/chat answers 409 — not 500.
func TestSessionBusyHTTP409(t *testing.T) {
	bus := messagebus.NewLocalBus()
	// Replica A owns a long-running turn.
	srvA := coordServer(t, bus)
	a := makeMockAgent(turnEvents("r1"), 5*time.Second)
	ch, err := srvA.sessionCoord.Run(context.Background(), "s-busy", a, turnMsg())
	require.NoError(t, err)
	defer drain(ch)

	// Replica B sees busy through the HTTP surface.
	srvB := coordServer(t, bus)
	w := postChat(t, srvB, "s-busy", "hi")
	require.Equal(t, http.StatusConflict, w.Code, "busy must map to 409, got %d %s", w.Code, w.Body.String())

	// A normal (non-busy) session still works through the same server.
	w = postChat(t, srvB, "s-free", "hi")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestSessionRunLockErrorNotBusy verifies that a broken bus must not
// masquerade as busy: the lock error keeps its own identity.
func TestSessionRunLockErrorNotBusy(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus).WithLockAcquireTimeout(50 * time.Millisecond)
	require.NoError(t, bus.Close())

	_, err := c.Run(context.Background(), "s1", makeMockAgent(turnEvents("r1"), 0), turnMsg())
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSessionBusy, "bus failure must not report busy")
}

// TestCrossProcessCancelHTTP verifies the acceptance clause: DELETE on a
// replica that does not own the run publishes the cancel request, and the
// owning replica terminates its run.
func TestCrossProcessCancelHTTP(t *testing.T) {
	bus := messagebus.NewLocalBus()
	srvA := coordServer(t, bus)
	a := makeMockAgent(turnEvents("r1"), 5*time.Second)
	ch, err := srvA.sessionCoord.Run(context.Background(), "s-x", a, turnMsg())
	require.NoError(t, err)

	// Replica B: no local run — DELETE must publish cross-process cancel.
	srvB := coordServer(t, bus)
	w := deleteChat(t, srvB, "s-x")
	require.Equal(t, http.StatusAccepted, w.Code, "remote cancel must be accepted, got %d %s", w.Code, w.Body.String())

	// A's run must terminate promptly.
	done := make(chan struct{})
	go func() {
		drain(ch)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cross-process cancel did not terminate the run in time")
	}
}

// TestCrossReplicaParked verifies the parked semantics from the OTHER
// replica's point of view: the registry marker is still present (run not
// exited) but the event log ends on a HITL request — status must be parked.
func TestCrossReplicaParked(t *testing.T) {
	bus := messagebus.NewLocalBus()
	srvA := coordServer(t, bus)
	// The suspended turn's event stream ends on the HITL request — that is
	// the cross-replica tail signal for parked.
	a := &parkedMockAgent{smMockAgent: smMockAgent{events: []event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewRequireUserConfirm("r1", "c1", nil),
	}, holdLast: true}, parked: true}
	ch, err := srvA.sessionCoord.Run(context.Background(), "s-p", a, turnMsg())
	require.NoError(t, err)
	// The held-open stream never closes on its own (parked at HITL): cancel
	// before draining, or the deferred drain would block forever.
	defer func() {
		_ = srvA.sessionCoord.Cancel(context.Background(), "s-p")
		drain(ch)
	}()

	// Replica B: registry says "running somewhere", but the log tail carries
	// the HITL request — parked wins over plain running.
	srvB := coordServer(t, bus)
	w := httptest.NewRequest("GET", "/api/v1/sessions/s-p/status", nil)
	rec := httptest.NewRecorder()
	srvB.ServeHTTP(rec, w)
	// The events must reach the log before the verdict is meaningful.
	require.Eventually(t, func() bool {
		rec = httptest.NewRecorder()
		srvB.ServeHTTP(rec, w)
		return rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "parked")
	}, 3*time.Second, 20*time.Millisecond, "cross-replica status must turn parked: last=%s", rec.Body.String())
}
