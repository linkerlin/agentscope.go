// Package wakeupapi is the 16.2 fourth registration-functionized cluster:
// the team-collaboration wakeup dispatcher. Dependencies are narrowed to
// small interfaces (Sessions: Run/IsActive) plus messagebus.TeamBus and
// service.Storage so the cluster does not import the gateway root; the root
// keeps a type alias and a thin constructor (see gateway/wakeup_compat.go).
//
// The dispatcher realises Python agentscope's WakeupDispatcher: a single
// background loop subscribes to the bus's wakeup signal stream and, for each
// idle session, drains its inbox of pending <team-message> blocks and
// re-runs the agent with them as input. Busy sessions are retried until
// idle (messages stay persisted in the inbox).
package wakeupapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// wakeupBusyTimeout caps how long a busy-session retry waits before giving up
// (leaving messages in the inbox for a future wakeup).
const wakeupBusyTimeout = 30 * time.Second

// wakeupPollInterval is the poll cadence while waiting for a busy session.
const wakeupPollInterval = 200 * time.Millisecond

// wakeupResumeInjectTimeout bounds how long the resume consumer waits for
// the restored turn to register its confirmation waiter.
const wakeupResumeInjectTimeout = 10 * time.Second

// Sessions starts session turns and reports liveness (satisfied structurally
// by the gateway root's *SessionManager).
type Sessions interface {
	Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)
	IsActive(sessionID string) bool
}

// RunFunc starts a session turn, usually through the coordinator
// (cross-replica single-consumer semantics, 18.5). nil means "use the
// injected Sessions".
type RunFunc func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)

// WakeupDispatcher drains team inboxes and re-runs idle worker sessions. It is
// the async collaboration engine: TeamSay/AgentCreate enqueue wakeups; this
// loop turns them into actual agent runs. Mirrors Python agentscope's
// app/_manager/_wakeup_dispatcher.py.
type WakeupDispatcher struct {
	bus      messagebus.TeamBus
	sessions Sessions
	storage  service.Storage
	// buildAgent constructs a fresh agent for a session (Server.buildSessionAgentFromStorage).
	buildAgent func(ctx context.Context, agentID, sessionID string) (agent.Agent, error)
	// run starts the turn; injected by Server.Start as the coordinator entry
	// (18.5): wakeup-driven turns then participate in cross-replica session
	// leases like HTTP-driven ones. nil falls back to sessions.Run.
	run RunFunc
	// busyRetryTimeout bounds startTurn's busy-retry window.
	busyRetryTimeout time.Duration

	cancel context.CancelFunc
	done   chan struct{}
}

// NewWakeupDispatcher creates a dispatcher. buildAgent should resolve to
// Server.buildSessionAgentFromStorage (or an equivalent per-session builder).
func NewWakeupDispatcher(
	bus messagebus.TeamBus,
	sm Sessions,
	storage service.Storage,
	buildAgent func(ctx context.Context, agentID, sessionID string) (agent.Agent, error),
) *WakeupDispatcher {
	return &WakeupDispatcher{
		bus:              bus,
		sessions:         sm,
		storage:          storage,
		buildAgent:       buildAgent,
		busyRetryTimeout: wakeupBusyTimeout,
	}
}

// WithRun routes wakeup-driven turns through fn (typically
// SessionCoordinator.Run). Without it the dispatcher uses the in-process
// Sessions only.
func (d *WakeupDispatcher) WithRun(fn RunFunc) *WakeupDispatcher {
	d.run = fn
	return d
}

// WithBusyRetryTimeout bounds how long startTurn retries a session reported
// busy by another replica (default wakeupBusyTimeout).
func (d *WakeupDispatcher) WithBusyRetryTimeout(dur time.Duration) *WakeupDispatcher {
	if dur > 0 {
		d.busyRetryTimeout = dur
	}
	return d
}

// Start launches the wakeup loop. It subscribes to the bus and spawns a handler
// goroutine per wakeup event. Safe to call once; call Stop to tear down.
func (d *WakeupDispatcher) Start(parentCtx context.Context) error {
	if d == nil || d.bus == nil {
		return fmt.Errorf("wakeup_dispatcher: bus is nil")
	}
	ctx, cancel := context.WithCancel(parentCtx)
	d.cancel = cancel
	d.done = make(chan struct{})

	wakeCh, wakeCancel, err := d.bus.SubscribeWakeup(ctx)
	if err != nil {
		cancel()
		return fmt.Errorf("wakeup_dispatcher: subscribe: %w", err)
	}
	go func() {
		defer close(d.done)
		defer wakeCancel()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-wakeCh:
				if !ok {
					return
				}
				// Handle concurrently so one slow session build never stalls others.
				go d.handleWakeup(ctx, ev.SessionID)
			}
		}
	}()
	return nil
}

// Stop tears down the wakeup loop and waits for it to exit.
func (d *WakeupDispatcher) Stop() {
	if d == nil || d.cancel == nil {
		return
	}
	d.cancel()
	if d.done != nil {
		<-d.done
	}
}

// handleWakeup routes a wakeup to either immediate processing or a busy-retry.
func (d *WakeupDispatcher) handleWakeup(ctx context.Context, sessionID string) {
	if sessionID == "" {
		return
	}
	if d.sessions != nil && d.sessions.IsActive(sessionID) {
		go d.waitAndRun(ctx, sessionID)
		return
	}
	d.drainAndRun(ctx, sessionID)
}

// waitAndRun polls until the session is idle (or times out), then processes.
func (d *WakeupDispatcher) waitAndRun(ctx context.Context, sessionID string) {
	deadline := time.Now().Add(wakeupBusyTimeout)
	ticker := time.NewTicker(wakeupPollInterval)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if d.sessions == nil || !d.sessions.IsActive(sessionID) {
				d.drainAndRun(ctx, sessionID)
				return
			}
		}
	}
	// Timed out: leave messages in the inbox for a future wakeup.
}

// drainAndRun is the core: consume a persisted resume command if one is
// waiting (23.2 closed loop), otherwise read the inbox, build the agent,
// assemble team messages, and kick off a run. Errors are non-fatal (logged
// via response text only).
func (d *WakeupDispatcher) drainAndRun(ctx context.Context, sessionID string) {
	// Orphan guard: session must still exist.
	se, err := d.storage.GetSession(ctx, sessionID)
	if err != nil || se == nil {
		return
	}
	// A persisted resume command outranks inbox content: the suspended turn
	// must continue before new input is folded in (18.5 worker closes the
	// 23.2 cross-replica loop).
	if d.consumePendingResume(ctx, se) {
		return
	}
	// Drain pending team messages.
	msgs, err := d.bus.InboxDrain(ctx, sessionID)
	if err != nil || len(msgs) == 0 {
		return
	}
	// Build the agent for this session.
	ag, err := d.buildAgent(ctx, se.AgentID, sessionID)
	if err != nil || ag == nil {
		d.requeue(ctx, sessionID, msgs)
		return
	}
	// Assemble inbox messages into a single user turn.
	var sb strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&sb, "<team-message from=%q>\n%s\n</team-message>\n\n", m.From, m.Content)
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent(sb.String()).Build()
	// Turns go through the coordinator when wired (18.5): the wakeup path
	// now participates in cross-replica session leases — a session running
	// on another replica reports busy and the messages are re-queued.
	ch, err := d.startTurn(ctx, sessionID, ag, msg)
	if err != nil {
		d.requeue(ctx, sessionID, msgs)
		return
	}
	if ch != nil {
		// A failed worker turn must reach the leader (PyV2 #2386 parity).
		go d.watchRunFailure(ctx, se, ch)
	}
}

// startTurn launches the turn through the injected run entry (coordinator)
// with bounded busy-retry: another replica running the session is expected to
// finish, so we wait briefly instead of dropping drained messages.
func (d *WakeupDispatcher) startTurn(ctx context.Context, sessionID string, ag agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	run := d.run
	if run == nil {
		run = d.sessions.Run
	}
	bo := newBackoff(wakeupPollInterval, 2*time.Second)
	deadline := time.Now().Add(d.busyRetryTimeout)
	if d.busyRetryTimeout <= 0 {
		deadline = time.Now().Add(wakeupBusyTimeout)
	}
	for {
		ch, err := run(ctx, sessionID, ag, msg)
		if err == nil {
			return ch, nil
		}
		if !errors.Is(err, sessionapi.ErrSessionBusy) || time.Now().After(deadline) {
			return nil, err
		}
		if !sleepCtx(ctx, bo.next()) {
			return nil, ctx.Err()
		}
	}
}

// requeue pushes drained messages back into the inbox and re-arms the wakeup
// so another (or later) dispatcher attempt picks them up — drained-but-lost
// messages must not disappear (at-least-once).
func (d *WakeupDispatcher) requeue(ctx context.Context, sessionID string, msgs []messagebus.TeamMessage) {
	if len(msgs) == 0 {
		return
	}
	for _, m := range msgs {
		_ = d.bus.InboxPush(ctx, sessionID, m)
	}
	_ = d.bus.EnqueueWakeup(ctx, sessionID)
}

// consumePendingResume restores a turn suspended on human-in-the-loop and
// delivers the persisted resume command (23.2 state machine's executing
// state) to it. It reports whether a resume was started; on success the
// inbox stays untouched for the next wakeup.
//
// Safety: the turn starts through the coordinator entry, so the fencing
// lease proves no other replica is still executing this session — the
// at-most-once guarantee of 23.2 holds for concurrent replicas, and the
// crash-takeover case is bounded by the lease TTL.
func (d *WakeupDispatcher) consumePendingResume(ctx context.Context, se *service.Session) bool {
	if d.storage == nil {
		return false
	}
	snap, err := d.storage.GetSnapshot(ctx, se.ID)
	if err != nil || snap == nil || snap.PendingResume == nil || snap.State == nil || snap.State.SuspendedAt == nil {
		return false
	}
	cmd := snap.PendingResume
	v2a, err := d.buildAgent(ctx, se.AgentID, se.ID)
	if err != nil || v2a == nil {
		return false
	}
	v2, ok := v2a.(agent.V2Agent)
	if !ok {
		return false
	}
	if err := v2.LoadState(snap.State); err != nil {
		return false
	}
	// ReplyStream detects the suspended runtime state and resumes the turn:
	// it re-emits RequireUserConfirm and registers the confirmation waiter
	// the command below is delivered to. The input message is a placeholder
	// — history is restored from the snapshot.
	placeholder := message.NewMsg().Role(message.RoleUser).TextContent("<resume-session/>").Build()
	ch, err := d.startTurn(ctx, se.ID, v2, placeholder)
	if err != nil {
		// Busy: the holder replica is alive — leave the command for it.
		return false
	}
	// Deliver the persisted command once the waiter exists (the waiter is
	// registered shortly after the turn starts; ErrNoWaiter means "try
	// again", not "gone").
	confirm := event.NewUserConfirmResult(cmd.ReplyID, cmd.ConfirmID, cmd.Decisions)
	go func() {
		deadline := time.Now().Add(wakeupResumeInjectTimeout)
		for time.Now().Before(deadline) {
			if err := v2.InjectEvent(ctx, confirm); err == nil {
				return
			}
			if !sleepCtx(ctx, 100*time.Millisecond) {
				return
			}
		}
	}()
	// Delete the snapshot only after the resumed turn completes (23.2:
	// delete-on-completion, auditable crash windows).
	go func() {
		for range ch {
		}
		_ = d.storage.DeleteSnapshot(context.Background(), se.ID)
	}()
	return true
}

// watchRunFailure consumes the run's event stream just far enough to detect a
// terminal error, then notifies the team leader. It returns after the first
// error (or stream end) to avoid holding the channel.
func (d *WakeupDispatcher) watchRunFailure(ctx context.Context, se *service.Session, ch <-chan event.AgentEvent) {
	for ev := range ch {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			d.notifyLeaderOfFailure(ctx, se, e.Err)
			return
		}
		if _, ok := ev.(*event.ReplyEndEvent); ok {
			return // clean finish — no notification needed
		}
	}
}

// notifyLeaderOfFailure pushes a <team-error> message into the leader's inbox
// and wakes it so the failure surfaces on the next leader turn.
func (d *WakeupDispatcher) notifyLeaderOfFailure(ctx context.Context, se *service.Session, runErr string) {
	if d.storage == nil || se == nil || se.TeamID == "" {
		return
	}
	team, err := d.storage.GetTeam(ctx, se.TeamID)
	if err != nil || team == nil || team.LeaderSessionID == "" || team.LeaderSessionID == se.ID {
		return
	}
	errMsg := runErr
	if len(errMsg) > 300 {
		errMsg = errMsg[:300]
	}
	_ = d.bus.InboxPush(ctx, team.LeaderSessionID, messagebus.TeamMessage{
		From: "worker:" + se.ID,
		Content: fmt.Sprintf("<team-error from=%q>\nworker turn failed: %s\n</team-error>",
			se.ID, errMsg),
	})
	_ = d.bus.EnqueueWakeup(ctx, team.LeaderSessionID)
}

// --- local backoff helpers (16.2 move; verbatim from the gateway root's
// worker.go, which keeps its own copies for the role-lease loop) ---

// backoff is exponential with a ceiling; reset returns to the base.
type backoff struct {
	base, cur, max time.Duration
}

func newBackoff(base, max time.Duration) backoff {
	return backoff{base: base, cur: base, max: max}
}

func (b *backoff) next() time.Duration {
	d := b.cur
	b.cur *= 2
	if b.cur > b.max {
		b.cur = b.max
	}
	return d
}

func (b *backoff) reset() { b.cur = b.base }

// sleepCtx waits for d or ctx.Done; it reports whether the wait elapsed
// (false means the context is done and the caller should exit).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
