package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func turnEvents(replyID string) []event.AgentEvent {
	return []event.AgentEvent{
		event.NewReplyStart(replyID, "mock"),
		event.NewTextBlockDelta(replyID, 0, "hello"),
		event.NewReplyEnd(replyID, "mock"),
	}
}

func turnMsg() *message.Msg {
	return message.NewMsg().Role(message.RoleUser).TextContent("go").Build()
}

// drain consumes a run channel to completion, returning all events.
func drain(ch <-chan event.AgentEvent) []event.AgentEvent {
	var out []event.AgentEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func TestSessionCoordinator_LocalDegradesToSessionManager(t *testing.T) {
	// No bus wired: the coordinator must behave exactly like SessionManager.
	c := NewSessionCoordinator(NewSessionManager())
	a := makeMockAgent(turnEvents("r1"), 0)
	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	evts := drain(ch)
	if len(evts) != 3 {
		t.Fatalf("local mode must pass through: %d events", len(evts))
	}
	// Replay without a bus is an explicit error, not a silent empty list.
	_, _, err = c.ReplayEvents(context.Background(), "s1", 0, 10)
	require.Error(t, err)
}

func TestSessionCoordinator_LockAllowsSequentialRuns(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	a := makeMockAgent(turnEvents("r1"), 0)

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	evts := drain(ch)
	require.Len(t, evts, 3)

	// The lock must be released once the run finished: a second run works.
	ch2, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err, "lock must be released after the run ends")
	drain(ch2)
}

func TestSessionCoordinator_LockBlocksConcurrentRuns(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus).WithLockAcquireTimeout(50 * time.Millisecond)
	// The turn outlasts the acquire window, so a concurrent Run must see busy.
	a := makeMockAgent(turnEvents("r1"), 300*time.Millisecond)

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)

	// A second coordinator (another replica sharing the bus) must see busy.
	c2 := NewSessionCoordinator(NewSessionManager()).WithBus(bus).WithLockAcquireTimeout(50 * time.Millisecond)
	_, err = c2.Run(context.Background(), "s1", a, turnMsg())
	require.ErrorIs(t, err, ErrSessionBusy)

	drain(ch)
	// After completion the other replica can take over.
	ch2, err := c2.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	drain(ch2)
}

func TestSessionCoordinator_EventLogReplay(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	a := makeMockAgent(turnEvents("r1"), 0)

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	drain(ch)

	evts, next, err := c.ReplayEvents(context.Background(), "s1", 0, 100)
	require.NoError(t, err)
	require.Len(t, evts, 3, "every event of the run must land in the log")
	assert.Equal(t, event.TypeReplyStart, evts[0].EventType())
	assert.Equal(t, event.TypeReplyEnd, evts[2].EventType())
	assert.Equal(t, int64(3), next)

	// Cursor pagination replays a suffix.
	suffix, next2, err := c.ReplayEvents(context.Background(), "s1", 2, 100)
	require.NoError(t, err)
	require.Len(t, suffix, 1)
	assert.Equal(t, int64(3), next2)
}

func TestSessionCoordinator_CrossProcessCancel(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	// Long-running turn on replica A.
	a := makeMockAgent(turnEvents("r1"), 5*time.Second)

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)

	// Replica B cancels without a local run: the request must reach A.
	c2 := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	require.NoError(t, c2.Cancel(context.Background(), "s1"))

	// A's stream must end promptly (well under the 5s turn duration).
	done := make(chan struct{})
	go func() {
		drain(ch)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("remote cancel did not terminate the run in time")
	}
}

func TestSessionCoordinator_Purge(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	a := makeMockAgent(turnEvents("r1"), 0)

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)
	drain(ch)

	require.NoError(t, c.Purge(context.Background(), "s1"))

	// Event log is gone; the local completed buffer too.
	_, next, err := c.ReplayEvents(context.Background(), "s1", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(0), next, "log must be empty after purge")
	// Subscribe now yields nothing (completed buffer cleared).
	assert.Empty(t, drain(c.sm.Subscribe("s1")))
}

func TestSessionCoordinator_BgTaskRegistryLifecycle(t *testing.T) {
	c := NewSessionCoordinator(NewSessionManager())
	bg := c.BgTasks()

	id, done := bg.Register("s1", "turn")
	tk, ok := bg.Get(id)
	require.True(t, ok)
	assert.Equal(t, BgTaskRunning, tk.Status)
	assert.Equal(t, "s1", tk.SessionID)

	done(errors.New("boom"))
	tk, _ = bg.Get(id)
	assert.Equal(t, BgTaskFailed, tk.Status)
	assert.Contains(t, tk.Err, "boom")
	assert.NotNil(t, tk.FinishedAt)

	id2, done2 := bg.Register("s1", "turn")
	done2(nil)
	// Newest first listing.
	list := bg.List("s1")
	require.Len(t, list, 2)
	assert.Equal(t, id2, list[0].ID)
	assert.Equal(t, BgTaskDone, list[0].Status)
	assert.Empty(t, bg.List("other-session"))
}

func TestSessionCoordinator_RunningRegistryMarker(t *testing.T) {
	bus := messagebus.NewLocalBus()
	cb := messagebus.AsCoordBus(bus)
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	a := makeMockAgent(turnEvents("r1"), 60*time.Millisecond)

	ch, err := c.Run(context.Background(), "s1", a, turnMsg())
	require.NoError(t, err)

	// While running, the registry holds the marker.
	time.Sleep(5 * time.Millisecond)
	raw, err := cb.RegistryGet(context.Background(), messagebus.Keys.SessionRunRegistryNS(), "s1")
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(raw), "task_id"), "marker must carry metadata: %s", raw)

	drain(ch)
	// After the run the marker is gone.
	_, err = cb.RegistryGet(context.Background(), messagebus.Keys.SessionRunRegistryNS(), "s1")
	assert.ErrorIs(t, err, messagebus.ErrNotFound)
}
