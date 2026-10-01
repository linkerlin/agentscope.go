package wakeupapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// fakeV2Agent is a no-op V2Agent whose ReplyStream closes immediately,
// letting Sessions.Run finish instantly in tests.
type fakeV2Agent struct{}

func (f *fakeV2Agent) Name() string { return "fake" }
func (f *fakeV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return msg, nil
}
func (f *fakeV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg)
	close(ch)
	return ch, nil
}
func (f *fakeV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return msg, nil
}
func (f *fakeV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	close(ch)
	return ch, nil
}
func (f *fakeV2Agent) LoadState(s *agent.AgentState) error   { return nil }
func (f *fakeV2Agent) SaveState() (*agent.AgentState, error) { return &agent.AgentState{}, nil }
func (f *fakeV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

// activeSessions is an in-memory Sessions double: a session is active from
// Run until its event stream is drained (mirrors SessionManager semantics).
type activeSessions struct {
	mu     sync.Mutex
	active map[string]bool
}

func newActiveSessions() *activeSessions { return &activeSessions{active: map[string]bool{}} }

func (f *activeSessions) Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	v2, ok := a.(agent.V2Agent)
	if !ok {
		return nil, fmt.Errorf("agent does not support V2 streaming")
	}
	ch, err := v2.ReplyStream(ctx, msg)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.active[sessionID] = true
	f.mu.Unlock()
	out := make(chan event.AgentEvent)
	go func() {
		defer close(out)
		for ev := range ch {
			out <- ev
		}
		f.mu.Lock()
		delete(f.active, sessionID)
		f.mu.Unlock()
	}()
	return out, nil
}

func (f *activeSessions) IsActive(sessionID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active[sessionID]
}

// seedWakeupSession persists a worker session + agent config.
func seedWakeupSession(t *testing.T, storage *service.MemoryStorage, sessionID string) {
	t.Helper()
	ctx := context.Background()
	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "wa", UserID: "u1", Name: "Worker", Source: "team"})
	_ = storage.SaveSession(ctx, &service.Session{ID: sessionID, UserID: "u1", AgentID: "wa", Source: "team"})
}

func TestWakeupDispatcher_DrainAndRun(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()

	// Seed a worker session + config (as AgentCreate would).
	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "wa", UserID: "u1", Name: "Worker", Source: "team"})
	_ = storage.SaveSession(ctx, &service.Session{ID: "ws", UserID: "u1", AgentID: "wa", Source: "team"})
	_ = bus.InboxPush(ctx, "ws", messagebus.TeamMessage{From: "Leader", Content: "do task"})

	buildCalled := false
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		buildCalled = true
		if agentID != "wa" || sessionID != "ws" {
			t.Errorf("unexpected build args: agent=%s session=%s", agentID, sessionID)
		}
		return &fakeV2Agent{}, nil
	})

	d.drainAndRun(ctx, "ws")

	if !buildCalled {
		t.Fatal("buildAgent was not called")
	}
	// Inbox must be drained after the run is kicked off.
	if msgs, _ := bus.InboxDrain(ctx, "ws"); len(msgs) != 0 {
		t.Fatalf("inbox should be empty after drainAndRun, got %d", len(msgs))
	}
}

func TestWakeupDispatcher_OrphanGuard(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()

	buildCalled := false
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		buildCalled = true
		return &fakeV2Agent{}, nil
	})
	// No session seeded -> orphan guard should skip.
	d.drainAndRun(ctx, "nonexistent")
	if buildCalled {
		t.Fatal("buildAgent should not be called for orphan session")
	}
}

// failingV2Agent emits a terminal ErrorEvent for every turn.
type failingV2Agent struct{}

func (failingV2Agent) Name() string { return "failing" }

func (failingV2Agent) Call(context.Context, *message.Msg) (*message.Msg, error) {
	return nil, nil
}

func (failingV2Agent) CallStream(context.Context, *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func (failingV2Agent) Reply(context.Context, *message.Msg) (*message.Msg, error) {
	return nil, nil
}

func (failingV2Agent) ReplyStream(_ context.Context, _ *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 2)
	ch <- event.NewError("r1", context.DeadlineExceeded)
	ch <- event.NewReplyEnd("r1", "failing")
	close(ch)
	return ch, nil
}

func (failingV2Agent) LoadState(*agent.AgentState) error     { return nil }
func (failingV2Agent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (failingV2Agent) InjectEvent(context.Context, event.AgentEvent) error {
	return nil
}

func TestWakeupDispatcher_WorkerFailureNotifiesLeader(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storage := service.NewMemoryStorage()
	if err := storage.SaveTeam(ctx, &service.Team{
		ID: "team-1", UserID: "u1", LeaderSessionID: "leader-s",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveSession(ctx, &service.Session{
		ID: "worker-s", UserID: "u1", AgentID: "worker", TeamID: "team-1",
		Source: "team", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	bus := messagebus.NewLocalBus()
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage,
		func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
			return failingV2Agent{}, nil
		})

	// Seed the worker inbox, then drain+run: the failing turn must surface in
	// the leader's inbox as a <team-error> message.
	if err := bus.InboxPush(ctx, "worker-s", messagebus.TeamMessage{From: "leader", Content: "do work"}); err != nil {
		t.Fatal(err)
	}
	d.drainAndRun(ctx, "worker-s")

	deadline := time.Now().Add(2 * time.Second)
	for {
		msgs, _ := bus.InboxDrain(ctx, "leader-s")
		for _, m := range msgs {
			if m.From == "worker:worker-s" && strings.HasPrefix(m.Content, "<team-error") {
				return // notified
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("leader was not notified of the worker failure")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// holdV2Agent keeps its stream open so the session stays active in tests.
type holdV2Agent struct {
	fakeV2Agent
	stream chan event.AgentEvent
}

func (h *holdV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	return h.stream, nil
}

// TestWakeupDispatcher_BusyRetry verifies the busy path: when a wakeup targets
// a session that is already running, the dispatcher waits for it to become
// idle (polling), then drains the inbox and runs. Messages are never lost.
func TestWakeupDispatcher_BusyRetry(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()

	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "wa", UserID: "u1", Name: "W", Source: "team"})
	_ = storage.SaveSession(ctx, &service.Session{ID: "ws", UserID: "u1", AgentID: "wa", Source: "team"})
	_ = bus.InboxPush(ctx, "ws", messagebus.TeamMessage{From: "L", Content: "do task"})

	sm := newActiveSessions()
	// Occupy the session with a holder whose stream never closes on its own.
	hold := &holdV2Agent{stream: make(chan event.AgentEvent)}
	holdMsg := message.NewMsg().Role(message.RoleUser).TextContent("hold").Build()
	if _, err := sm.Run(ctx, "ws", hold, holdMsg); err != nil {
		t.Fatal(err)
	}
	if !sm.IsActive("ws") {
		t.Fatal("session should be active while holder stream is open")
	}

	var buildCalled atomic.Bool
	d := NewWakeupDispatcher(bus, sm, storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		buildCalled.Store(true)
		return &fakeV2Agent{}, nil
	})

	// Wakeup hits a busy session -> spawns a background retry; build must not
	// happen yet.
	d.handleWakeup(ctx, "ws")
	time.Sleep(300 * time.Millisecond)
	if buildCalled.Load() {
		t.Fatal("buildAgent must not run while the session is busy")
	}

	// Release the holder: closing the stream lets the run finish -> idle.
	close(hold.stream)

	// The retry loop should detect idle and process the inbox.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !buildCalled.Load() {
		time.Sleep(50 * time.Millisecond)
	}
	if !buildCalled.Load() {
		t.Fatal("buildAgent not called after session became idle (busy retry failed)")
	}
	if msgs, _ := bus.InboxDrain(ctx, "ws"); len(msgs) != 0 {
		t.Fatalf("inbox should be drained after busy retry, got %d", len(msgs))
	}
}

// TestWakeupDispatcherUsesCoordinatedRunAndRequeuesOnBusy: turns go through
// the injected (coordinated) run entry; a session busy on another replica
// for the whole retry window re-queues the drained messages instead of
// losing them.
func TestWakeupDispatcherUsesCoordinatedRunAndRequeuesOnBusy(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()
	seedWakeupSession(t, storage, "ws")

	_ = bus.InboxPush(ctx, "ws", messagebus.TeamMessage{From: "Leader", Content: "do task"})

	var runCalls atomic.Int32
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return &fakeV2Agent{}, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			runCalls.Add(1)
			// Another replica holds the session: busy for the whole window.
			return nil, fmt.Errorf("%w: %s", sessionapi.ErrSessionBusy, sessionID)
		}).
		WithBusyRetryTimeout(150 * time.Millisecond)

	d.drainAndRun(ctx, "ws")

	if runCalls.Load() < 2 {
		t.Fatalf("busy retry did not happen, run calls=%d", runCalls.Load())
	}
	// The drained messages must be back in the inbox with a re-armed wakeup.
	msgs, err := bus.InboxDrain(ctx, "ws")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Content != "do task" {
		t.Fatalf("messages not re-queued: %+v", msgs)
	}
}

// TestWakeupDispatcherBusyThenSuccess: a busy window shorter than the retry
// timeout resolves into a successful run.
func TestWakeupDispatcherBusyThenSuccess(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()
	seedWakeupSession(t, storage, "ws2")
	_ = bus.InboxPush(ctx, "ws2", messagebus.TeamMessage{From: "Leader", Content: "hello"})

	var calls atomic.Int32
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return &fakeV2Agent{}, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			if calls.Add(1) == 1 {
				return nil, fmt.Errorf("%w: %s", sessionapi.ErrSessionBusy, sessionID)
			}
			ch := make(chan event.AgentEvent)
			close(ch)
			return ch, nil
		})
	d.drainAndRun(ctx, "ws2")

	if calls.Load() != 2 {
		t.Fatalf("expected busy then success, calls=%d", calls.Load())
	}
	if msgs, _ := bus.InboxDrain(ctx, "ws2"); len(msgs) != 0 {
		t.Fatalf("inbox must stay empty after a successful run, got %+v", msgs)
	}
}

// resumeFakeAgent records LoadState/InjectEvent so the resume-consumption
// test can observe the 23.2 closed loop without a real ReActAgent.
type resumeFakeAgent struct {
	fakeV2Agent
	loaded    atomic.Bool
	injected  atomic.Bool
	injectErr error
}

func (f *resumeFakeAgent) LoadState(s *agent.AgentState) error {
	f.loaded.Store(true)
	return nil
}

func (f *resumeFakeAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	if f.injectErr != nil {
		return f.injectErr
	}
	f.injected.Store(true)
	return nil
}

// TestWakeupDispatcherConsumesPendingResume: a persisted resume command
// (23.2's executing state, no waiter on this replica) is consumed here — the
// agent is rebuilt from the snapshot, the turn restarts through the
// coordinated entry, the command is delivered, and the snapshot is deleted
// only after the turn completes.
func TestWakeupDispatcherConsumesPendingResume(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()
	seedWakeupSession(t, storage, "rs")

	// Suspended snapshot with a persisted (executing) resume command.
	suspended := time.Now().UTC()
	_ = storage.SaveSnapshot(ctx, &service.AgentSnapshot{
		SessionID: "rs",
		ReplyID:   "reply-1",
		State: &agent.AgentState{
			Version:     "v2",
			ReplyID:     "reply-1",
			SuspendedAt: &suspended,
		},
		PendingResume: &service.ResumeCommand{
			ConfirmID: "cf-1",
			ReplyID:   "reply-1",
			State:     service.ResumeExecuting,
			Version:   1,
		},
	})
	// Inbox content that must NOT be folded in while the resume is pending.
	_ = bus.InboxPush(ctx, "rs", messagebus.TeamMessage{From: "L", Content: "later"})

	fa := &resumeFakeAgent{}
	runDone := make(chan struct{})
	var runCalled atomic.Bool
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return fa, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			runCalled.Store(true)
			ch := make(chan event.AgentEvent, 1)
			ch <- event.NewReplyEnd("reply-1", "Worker")
			close(ch)
			close(runDone)
			return ch, nil
		})
	d.drainAndRun(ctx, "rs")

	if !runCalled.Load() {
		t.Fatal("resume turn not started through the coordinated entry")
	}
	if !fa.loaded.Load() {
		t.Fatal("snapshot state not loaded into the rebuilt agent")
	}
	// Command delivery is asynchronous (waits for the waiter); poll briefly.
	deadline := time.Now().Add(2 * time.Second)
	for !fa.injected.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !fa.injected.Load() {
		t.Fatal("persisted resume command never delivered")
	}
	// Inbox untouched: the pending resume outranks inbox content.
	if msgs, _ := bus.InboxDrain(ctx, "rs"); len(msgs) != 1 {
		t.Fatalf("inbox must keep its message during resume, got %+v", msgs)
	}
	// Snapshot deleted after the turn completes (23.2 delete-on-completion).
	deadline = time.Now().Add(2 * time.Second)
	for {
		if _, err := storage.GetSnapshot(ctx, "rs"); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("snapshot not deleted after the resumed turn completed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// A second wakeup now drains the inbox normally (no pending resume).
	d2 := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return &fakeV2Agent{}, nil
	})
	d2.drainAndRun(ctx, "rs")
	if msgs, _ := bus.InboxDrain(ctx, "rs"); len(msgs) != 0 {
		t.Fatalf("second wakeup should have drained the inbox, got %+v", msgs)
	}
}

// TestWakeupDispatcherSkipsResumeWhenHolderBusy: the resume command is left
// on the snapshot when the coordinator reports the session busy on another
// replica (the holder will consume it).
func TestWakeupDispatcherSkipsResumeWhenHolderBusy(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()
	seedWakeupSession(t, storage, "bs")
	suspended := time.Now().UTC()
	_ = storage.SaveSnapshot(ctx, &service.AgentSnapshot{
		SessionID: "bs",
		ReplyID:   "reply-1",
		State:     &agent.AgentState{Version: "v2", ReplyID: "reply-1", SuspendedAt: &suspended},
		PendingResume: &service.ResumeCommand{
			ConfirmID: "cf-2", ReplyID: "reply-1", State: service.ResumeExecuting,
		},
	})

	fa := &resumeFakeAgent{}
	d := NewWakeupDispatcher(bus, newActiveSessions(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return fa, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			return nil, fmt.Errorf("%w: %s", sessionapi.ErrSessionBusy, sessionID)
		}).
		WithBusyRetryTimeout(50 * time.Millisecond)
	d.drainAndRun(ctx, "bs")

	// LoadState on a freshly built local agent is harmless (the instance is
	// discarded); what must NOT happen is delivering the command or touching
	// the snapshot while the holder replica is alive.
	if fa.injected.Load() {
		t.Fatal("resume command delivered while another replica holds the session")
	}
	// Snapshot (and its command) must survive for the holder.
	if _, err := storage.GetSnapshot(ctx, "bs"); err != nil {
		t.Fatal("snapshot dropped while the holder replica is alive")
	}
}
