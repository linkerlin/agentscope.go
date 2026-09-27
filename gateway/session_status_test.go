package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parkedMockAgent reports a HITL suspension via its runtime state.
type parkedMockAgent struct {
	smMockAgent
	parked bool
}

func (m *parkedMockAgent) SaveState() (*agent.AgentState, error) {
	st := &agent.AgentState{}
	if m.parked {
		now := time.Now()
		st.SuspendedAt = &now
	}
	return st, nil
}

func TestSessionCoordinator_Status_UnknownIdleRunning(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	ctx := context.Background()

	// No record anywhere.
	assert.Equal(t, StatusUnknown, c.Status(ctx, "ghost"))

	// A completed run leaves history -> idle (event log + completed buffer).
	a := makeMockAgent(turnEvents("r1"), 0)
	ch, err := c.Run(ctx, "s1", a, turnMsg())
	require.NoError(t, err)
	drain(ch)
	assert.Equal(t, StatusIdle, c.Status(ctx, "s1"))

	// A running turn -> running, on this replica and from another replica.
	slow := makeMockAgent(turnEvents("r2"), 300*time.Millisecond)
	ch2, err := c.Run(ctx, "s2", slow, turnMsg())
	require.NoError(t, err)
	defer drain(ch2)
	assert.Equal(t, StatusRunning, c.Status(ctx, "s2"))

	c2 := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	assert.Equal(t, StatusRunning, c2.Status(ctx, "s2"), "registry marker must make the run visible cross-replica")
}

func TestSessionCoordinator_Status_ParkedWhileSuspended(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	a := &parkedMockAgent{smMockAgent: smMockAgent{events: turnEvents("r1"), delay: 300 * time.Millisecond}, parked: true}

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	defer drain(ch)

	assert.Equal(t, StatusParked, c.Status(context.Background(), "s1"),
		"an active run whose agent is suspended at HITL must report parked")
}

func TestSessionCoordinator_Status_ParkedFromEventLogTail(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	// The run ends with a HITL request as its last logged event.
	a := makeMockAgent([]event.AgentEvent{
		event.NewReplyStart("r1", "mock"),
		event.NewRequireUserConfirm("r1", "c1", nil),
	}, 0)
	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	drain(ch)

	// Clear the local completed buffer to prove the verdict comes from the
	// event log tail (cross-replica data path), not from local state.
	c.sm.ClearCompleted("s1")
	assert.Equal(t, StatusParked, c.Status(context.Background(), "s1"))
}

// TestSessionStatusEndpoint covers the HTTP surface: 200 + JSON shape via the
// local (no-coordinator) degradation path.
func TestSessionStatusEndpoint(t *testing.T) {
	srv := NewServer(&mockAgent{})
	srv.WithSessionManager(NewSessionManager())

	sessID := "sess-status"
	a := makeMockAgent(turnEvents("r1"), 0)
	ch, err := srv.sessionMgr.Run(context.Background(), sessID, a, turnMsg())
	require.NoError(t, err)
	drain(ch)

	srv.RegisterV2Routes()
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessID+"/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, sessID, resp.SessionID)
	assert.Equal(t, "idle", resp.Status)

	// Unknown session.
	req = httptest.NewRequest("GET", "/api/v1/sessions/nope/status", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "unknown", resp.Status)
}
