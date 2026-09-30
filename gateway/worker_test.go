package gateway

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// --- backoff ---

func TestBackoffSequence(t *testing.T) {
	b := newBackoff(100*time.Millisecond, 800*time.Millisecond)
	seq := []time.Duration{b.next(), b.next(), b.next(), b.next()}
	want := []time.Duration{100, 200, 400, 800}
	for i := range want {
		if seq[i] != want[i]*time.Millisecond {
			t.Fatalf("next[%d]=%v, want %v", i, seq[i], want[i]*time.Millisecond)
		}
	}
	// Capped.
	if got := b.next(); got != 800*time.Millisecond {
		t.Fatalf("cap not applied: %v", got)
	}
	b.reset()
	if got := b.next(); got != 100*time.Millisecond {
		t.Fatalf("reset did not return to base: %v", got)
	}
}

// --- Worker role lease (single holder + takeover, 18.5) ---

func workerTestRunner(role WorkerRole, started chan<- string, stopped *atomic.Int32) RoleRunner {
	return RoleRunner{
		Role: role,
		Start: func() error {
			started <- string(role)
			return nil
		},
		Stop: func() { stopped.Add(1) },
	}
}

// TestWorkerSingleHolderAndTakeover locks the competitive-acquire semantics:
// two workers over one bus, only one holds the role; stopping the holder
// releases the lease and the standby takes over.
func TestWorkerSingleHolderAndTakeover(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	lease := messagebus.AsCoordLease(bus)
	if lease == nil {
		t.Fatal("LocalBus must implement CoordLease")
	}

	started := make(chan string, 8)
	var stopped1, stopped2 atomic.Int32
	w1 := NewWorker(lease, "worker-1", []RoleRunner{workerTestRunner(RoleWakeup, started, &stopped1)}).
		WithLeaseTTL(80 * time.Millisecond)
	w2 := NewWorker(lease, "worker-2", []RoleRunner{workerTestRunner(RoleWakeup, started, &stopped2)}).
		WithLeaseTTL(80 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w1.Start(ctx)
	// W1 acquires first.
	select {
	case r := <-started:
		if r != string(RoleWakeup) {
			t.Fatalf("unexpected role started: %s", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker-1 never acquired the role")
	}

	w2.Start(ctx)
	// While W1 holds (heartbeat keeps renewing), W2 must NOT start.
	select {
	case r := <-started:
		t.Fatalf("worker-2 started concurrently: %s", r)
	case <-time.After(300 * time.Millisecond):
	}

	// Graceful stop releases the lease; W2 takes over.
	w1.Stop()
	if stopped1.Load() != 1 {
		t.Fatalf("worker-1 component not stopped, count=%d", stopped1.Load())
	}
	select {
	case r := <-started:
		if r != string(RoleWakeup) {
			t.Fatalf("unexpected takeover role: %s", r)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-2 never took over after release")
	}
	w2.Stop()
	if stopped2.Load() != 1 {
		t.Fatalf("worker-2 component not stopped, count=%d", stopped2.Load())
	}
}

// losingLease hands out one lease then reports every renewal as lost and
// every later acquire as held — deterministically exercising the
// lose → stop → re-acquire path without TTL timing.
type losingLease struct {
	mu        sync.Mutex
	acquired  bool
	holdCount atomic.Int32
}

func (l *losingLease) AcquireLease(ctx context.Context, key, owner string, ttl time.Duration) (*messagebus.Lease, messagebus.Cancel, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.acquired {
		return nil, nil, messagebus.ErrLeaseHeld
	}
	l.acquired = true
	return &messagebus.Lease{Key: key, Owner: owner, Token: 1, Expires: time.Now().Add(ttl)},
		func() {}, nil
}

func (l *losingLease) RenewLease(ctx context.Context, key string, ls *messagebus.Lease, ttl time.Duration) error {
	l.holdCount.Add(1)
	return messagebus.ErrLeaseLost
}

// TestWorkerLosesLeaseStopsComponent: a failed heartbeat renewal (fence
// superseded — crash takeover elsewhere) must stop the component
// immediately.
func TestWorkerLosesLeaseStopsComponent(t *testing.T) {
	fl := &losingLease{}
	started := make(chan string, 4)
	var stopped atomic.Int32
	w := NewWorker(fl, "worker-x", []RoleRunner{workerTestRunner(RoleSchedule, started, &stopped)}).
		WithLeaseTTL(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	defer w.Stop()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never acquired the role")
	}
	// Heartbeat fires immediately after start; the lost lease must stop the
	// component.
	deadline := time.Now().Add(2 * time.Second)
	for stopped.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if stopped.Load() != 1 {
		t.Fatal("component not stopped after lease loss")
	}
	cancel()
}

// --- wakeup dispatcher: coordinated run, busy requeue, resume consumption ---

// seedWakeupSession persists a worker session + agent config.
func seedWakeupSession(t *testing.T, storage *service.MemoryStorage, sessionID string) {
	t.Helper()
	ctx := context.Background()
	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "wa", UserID: "u1", Name: "Worker", Source: "team"})
	_ = storage.SaveSession(ctx, &service.Session{ID: sessionID, UserID: "u1", AgentID: "wa", Source: "team"})
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
	d := NewWakeupDispatcher(bus, NewSessionManager(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return &fakeV2Agent{}, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			runCalls.Add(1)
			// Another replica holds the session: busy for the whole window.
			return nil, fmt.Errorf("%w: %s", ErrSessionBusy, sessionID)
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
	d := NewWakeupDispatcher(bus, NewSessionManager(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return &fakeV2Agent{}, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			if calls.Add(1) == 1 {
				return nil, fmt.Errorf("%w: %s", ErrSessionBusy, sessionID)
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
	d := NewWakeupDispatcher(bus, NewSessionManager(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
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
	d2 := NewWakeupDispatcher(bus, NewSessionManager(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
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
	d := NewWakeupDispatcher(bus, NewSessionManager(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		return fa, nil
	}).
		WithRun(func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
			return nil, fmt.Errorf("%w: %s", ErrSessionBusy, sessionID)
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

// --- channel runner busy retry ---

// TestChannelRunnerBusyRetryThenRun: a busy session on another replica is
// retried with backoff; the reply flows back once the turn runs locally.
func TestChannelRunnerBusyRetryThenRun(t *testing.T) {
	storage := service.NewMemoryStorage()
	_ = storage.SaveAgentConfig(context.Background(), &service.AgentConfig{ID: "ca", UserID: "u1", Name: "C"})
	reg := NewAgentRegistry()
	reg.Register("ca", &fakeV2Agent{})
	sm := NewSessionManager().WithStorage(storage)
	r := NewChannelRunner(reg, sm)

	var calls atomic.Int32
	r.Run = func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("%w: %s", ErrSessionBusy, sessionID)
		}
		ch := make(chan event.AgentEvent, 1)
		ch <- event.NewTextBlockDelta("reply-1", 0, "hello back")
		close(ch)
		return ch, nil
	}
	sent := make(chan string, 1)
	r.Lookup = func(channelID string) channel.Channel {
		return fakeSendChannel{send: func(chatID, text string) { sent <- text }}
	}

	if err := r.RunUserTurn(context.Background(), "ca", "cs", channel.ChannelEvent{
		ChannelID: "ch1", ChatID: "chat1", ChannelUserID: "u9", Text: "hi",
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case text := <-sent:
		if text != "hello back" {
			t.Fatalf("unexpected reply: %q", text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reply never sent")
	}
	if calls.Load() != 2 {
		t.Fatalf("expected busy-then-run, calls=%d", calls.Load())
	}
}

// --- server-level split deployment (18.5) ---

// TestServerWorkerRoleSelection locks the role selection at assembly level:
// an explicit empty set means no long-running loops (API replica), while the
// default set runs them lease-guarded over the shared bus.
func TestServerWorkerRoleSelection(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	storage := service.NewMemoryStorage()
	seedWakeupSession(t, storage, "split-ws")

	// API replica: explicitly no roles.
	apiSrv := NewServer(&fakeV2Agent{}).
		WithStorage(storage).
		WithSessionManager(NewSessionManager().WithStorage(storage))
	apiSrv.WithMessageBus(bus)
	apiSrv.WithWorkerRoles() // none
	apiSrv.Start()
	defer apiSrv.Close()
	if apiSrv.worker != nil {
		t.Fatal("API replica must not run any worker role")
	}

	// Worker replica: default role set over the lease-capable bus.
	wSrv := NewServer(&fakeV2Agent{}).
		WithStorage(storage).
		WithSessionManager(NewSessionManager().WithStorage(storage))
	wSrv.WithMessageBus(bus)
	wSrv.Start()
	defer wSrv.Close()

	if wSrv.worker == nil {
		t.Fatal("lease-capable bus must guard roles with a Worker")
	}
	// Behavioural proof the wakeup role is live: an inbox message plus a
	// wakeup signal gets drained and run through the worker's dispatcher.
	_ = bus.InboxPush(context.Background(), "split-ws", messagebus.TeamMessage{From: "L", Content: "run"})
	_ = bus.EnqueueWakeup(context.Background(), "split-ws")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if msgs, _ := bus.InboxDrain(context.Background(), "split-ws"); len(msgs) == 0 {
			return // drained: the wakeup role is live
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("worker replica never drained the wakeup inbox")
}

// fakeSendChannel is a channel.Channel that only supports SendText.
type fakeSendChannel struct {
	send func(chatID, text string)
}

func (f fakeSendChannel) ID() string { return "fake" }
func (f fakeSendChannel) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
	<-ctx.Done()
	return nil
}
func (f fakeSendChannel) SendText(ctx context.Context, chatID, text string) error {
	f.send(chatID, text)
	return nil
}
func (f fakeSendChannel) Close() error { return nil }
