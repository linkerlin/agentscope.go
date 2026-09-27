package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/sessionapi"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
)

// SessionCoordinator layers cross-replica coordination (18.1) on top of the
// in-process SessionManager:
//
//   - SessionRun lock: at most one replica executes a session's turn. The
//     lock is held on the CoordBus for the whole run; a replica that cannot
//     acquire it gets ErrSessionBusy (map to HTTP 409).
//   - Event log: every event of a coordinated run is appended to the bus
//     log, so any replica can replay a session's history via ReplayEvents.
//   - Cross-process cancel: Cancel publishes on the session's cancel channel;
//     the replica holding the run subscribes during the run and terminates it.
//   - Purge: releases coordination state (running registry entry, event log,
//     completed replay buffer).
//   - BgTask registry: records background tasks spawned by session runs
//     (the turn itself is recorded; tool-offload and worker tasks register
//     through it as 18.5 lands).
//
// With a nil CoordBus (no bus configured, or a bus without coordination
// primitives) the coordinator degrades to the plain in-process SessionManager
// — single-replica behavior is unchanged.
type SessionCoordinator struct {
	sm      *SessionManager
	bus     messagebus.Bus
	cb      messagebus.CoordBus
	lockTTL time.Duration
	// acquireTimeout bounds the wait for the run lock (try-lock semantics).
	acquireTimeout time.Duration
	keys           messagebus.CoordKeys
	bg             *BgTaskRegistry
}

// ErrSessionBusy is returned by Run when another replica currently holds the
// session's run lock. Callers typically map it to HTTP 409 Conflict.
// Defined in sessionapi (the HTTP face maps it); aliased here.
var ErrSessionBusy = sessionapi.ErrSessionBusy

// DefaultSessionLockTTL guards against a crashed replica holding the lock
// forever. Normal completion releases the lock explicitly; the TTL only
// matters when a replica dies mid-run.
const DefaultSessionLockTTL = 30 * time.Minute

// DefaultSessionLockAcquireTimeout bounds how long Run waits for the run
// lock. CoordBus.Lock blocks until acquired or ctx is done, so the acquire
// timeout turns "another replica is running" into a fast ErrSessionBusy
// (HTTP 409) instead of an unbounded queue.
const DefaultSessionLockAcquireTimeout = 100 * time.Millisecond

// NewSessionCoordinator wraps sm with coordination. Coordination activates
// once WithBus supplies a bus carrying CoordBus primitives.
func NewSessionCoordinator(sm *SessionManager) *SessionCoordinator {
	return &SessionCoordinator{
		sm:             sm,
		lockTTL:        DefaultSessionLockTTL,
		acquireTimeout: DefaultSessionLockAcquireTimeout,
		keys:           messagebus.Keys,
		bg:             NewBgTaskRegistry(),
	}
}

// WithLockAcquireTimeout overrides how long Run waits for the run lock
// before reporting ErrSessionBusy. Zero or negative means wait unbounded.
func (c *SessionCoordinator) WithLockAcquireTimeout(d time.Duration) *SessionCoordinator {
	c.acquireTimeout = d
	return c
}

// WithBus wires the message bus used for cross-replica coordination. Buses
// without CoordBus primitives keep the coordinator in local mode.
func (c *SessionCoordinator) WithBus(b messagebus.Bus) *SessionCoordinator {
	c.bus = b
	c.cb = messagebus.AsCoordBus(b)
	return c
}

// WithLockTTL overrides the session-run lock TTL (crash protection window).
func (c *SessionCoordinator) WithLockTTL(d time.Duration) *SessionCoordinator {
	c.lockTTL = d
	return c
}

// SessionManager exposes the wrapped in-process manager (subscribe, steer and
// friends keep working locally).
func (c *SessionCoordinator) SessionManager() *SessionManager { return c.sm }

// BgTasks exposes the background-task registry.
func (c *SessionCoordinator) BgTasks() *BgTaskRegistry { return c.bg }

// coordinated reports whether cross-replica mode is active.
func (c *SessionCoordinator) coordinated() bool { return c.cb != nil }

// Run executes a session turn under the coordination protocol. See the type
// documentation for the lock / log / cancel semantics.
func (c *SessionCoordinator) Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	if !c.coordinated() {
		return c.sm.Run(ctx, sessionID, a, msg)
	}

	// Acquire the distributed lock on a detached context: the run must keep
	// its lock even when the triggering HTTP request goes away (SSE drop);
	// it is released when the run completes or by the TTL after a crash.
	// The acquire deadline turns "another replica is running" into a fast
	// ErrSessionBusy instead of an unbounded wait.
	lockCtx, lockCancel := context.WithCancel(context.Background())
	var acquireCancel context.CancelFunc
	if c.acquireTimeout > 0 {
		lockCtx, acquireCancel = context.WithTimeout(lockCtx, c.acquireTimeout)
	}
	release, err := c.cb.Lock(lockCtx, c.keys.SessionRunLockKey(sessionID), c.lockTTL)
	if err != nil {
		if acquireCancel != nil {
			acquireCancel()
		}
		lockCancel()
		// Only a genuine acquire timeout means "another replica is running";
		// bus failures (closed, Redis down) must NOT masquerade as busy.
		if c.acquireTimeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %s", ErrSessionBusy, sessionID)
		}
		return nil, fmt.Errorf("session coordinator: run lock: %w", err)
	}

	taskID, _ := c.bg.Register(sessionID, "turn")

	// Publish the running marker so other replicas can query/observe it.
	marker, _ := json.Marshal(map[string]any{
		"started_at": time.Now().UTC().Format(time.RFC3339),
		"task_id":    taskID,
	})
	_ = c.cb.RegistrySet(context.Background(), c.keys.SessionRunRegistryNS(), sessionID, marker)

	// Subscribe to the cross-process cancel channel for the whole run.
	cancelSubCtx, cancelSubCancel := context.WithCancel(context.Background())
	cancelCh, cancelCancel, _ := c.bus.Subscribe(cancelSubCtx, c.keys.SessionCancelChannel(sessionID))

	ch, err := c.sm.Run(ctx, sessionID, a, msg)
	if err != nil {
		cancelCancel()
		cancelSubCancel()
		if acquireCancel != nil {
			acquireCancel()
		}
		_ = c.cb.RegistryDelete(context.Background(), c.keys.SessionRunRegistryNS(), sessionID)
		c.bg.MarkDone(taskID, err)
		release()
		lockCancel()
		return nil, err
	}

	out := make(chan event.AgentEvent, 64)
	go func() {
		defer close(out)
		defer func() {
			cancelCancel()
			cancelSubCancel()
			_ = c.cb.RegistryDelete(context.Background(), c.keys.SessionRunRegistryNS(), sessionID)
			c.bg.MarkDone(taskID, nil)
			release()
			if acquireCancel != nil {
				acquireCancel()
			}
			lockCancel()
		}()

		// Cancel pump: a remote cancel terminates the run through the local
		// manager (agent interrupt + context cancel).
		go func() {
			for range cancelCh {
				_ = c.sm.Terminate(sessionID)
			}
		}()

		logNS := c.keys.SessionEventLogNS(sessionID)
		for ev := range ch {
			if ev != nil {
				if data, merr := event.MarshalEvent(ev); merr == nil {
					_, _ = c.cb.LogAppend(context.Background(), logNS, data)
				}
				out <- ev
			}
		}
	}()
	return out, nil
}

// Cancel stops a session's running turn. It terminates locally when this
// replica owns the run; otherwise it publishes the cancel request so the
// owning replica terminates it (cross-process cancel).
func (c *SessionCoordinator) Cancel(ctx context.Context, sessionID string) error {
	if c.sm.Terminate(sessionID) {
		return nil
	}
	if c.bus == nil {
		return fmt.Errorf("session coordinator: no local run for session %s", sessionID)
	}
	return c.bus.Publish(ctx, c.keys.SessionCancelChannel(sessionID), []byte("cancel"))
}

// ReplayEvents returns up to limit logged events of the session starting at
// cursor (0-based), plus the next cursor. Works from any replica sharing the
// bus; cursor 0 replays the whole history.
func (c *SessionCoordinator) ReplayEvents(ctx context.Context, sessionID string, cursor int64, limit int) ([]event.AgentEvent, int64, error) {
	if !c.coordinated() {
		// Local fallback: replay whatever the in-process buffers hold.
		return nil, 0, errors.New("session coordinator: event replay requires a coordination bus")
	}
	entries, next, err := c.cb.LogRead(ctx, c.keys.SessionEventLogNS(sessionID), cursor, limit)
	if err != nil {
		return nil, cursor, err
	}
	evts := make([]event.AgentEvent, 0, len(entries))
	for _, raw := range entries {
		ev, uerr := event.UnmarshalEvent(raw)
		if uerr != nil {
			// Skip undecodable entries rather than failing the whole replay;
			// the log is a best-effort durability surface.
			continue
		}
		evts = append(evts, ev)
	}
	return evts, next, nil
}

// Purge releases all coordination state of a session: the running registry
// marker, the event log, and the local completed replay buffer. The session
// must not be running.
func (c *SessionCoordinator) Purge(ctx context.Context, sessionID string) error {
	if c.coordinated() {
		if err := c.cb.RegistryDelete(ctx, c.keys.SessionRunRegistryNS(), sessionID); err != nil {
			return err
		}
		if err := c.cb.LogPurge(ctx, c.keys.SessionEventLogNS(sessionID)); err != nil {
			return err
		}
	}
	c.sm.ClearCompleted(sessionID)
	return nil
}

// IsActive reports whether the session currently runs on this replica.
func (c *SessionCoordinator) IsActive(sessionID string) bool { return c.sm.IsActive(sessionID) }

// --- Status (18.2) ---

// SessionStatus and its states live in gateway/sessionapi (the session HTTP
// face owns the wire vocabulary); aliases keep the root's public surface.
type SessionStatus = sessionapi.SessionStatus

const (
	// StatusRunning: a turn is executing on this or another replica.
	StatusRunning = sessionapi.StatusRunning
	// StatusParked: the session is suspended awaiting a human-in-the-loop
	// decision (tool confirmation or external execution).
	StatusParked = sessionapi.StatusParked
	// StatusIdle: the session has history but nothing is running or parked.
	StatusIdle = sessionapi.StatusIdle
	// StatusUnknown: no record of this session anywhere.
	StatusUnknown = sessionapi.StatusUnknown
)

// Status resolves the session's lifecycle state (18.2). Running wins over
// parked: a replica actively executing beats the parked marker of a stale
// suspension. Parked is detected locally via the agent's suspended runtime
// state, or remotely via the last logged event being a HITL request.
func (c *SessionCoordinator) Status(ctx context.Context, sessionID string) SessionStatus {
	if c.sm.IsActive(sessionID) {
		if c.sm.Suspended(sessionID) {
			return StatusParked
		}
		return StatusRunning
	}
	if c.coordinated() {
		// Another replica may hold the run. If its event log already ends on
		// a HITL request, the run is parked there: the registry marker stays
		// until the run exits, so the log tail must be consulted before
		// declaring plain running.
		_, rerr := c.cb.RegistryGet(ctx, c.keys.SessionRunRegistryNS(), sessionID)
		if rerr == nil {
			switch c.lastLoggedEventType(ctx, sessionID) {
			case event.TypeRequireUserConfirm, event.TypeRequireExternalExecution:
				return StatusParked
			}
			return StatusRunning
		}
		// No live run anywhere: a finished turn that ended on a HITL request
		// still parks the session.
		switch c.lastLoggedEventType(ctx, sessionID) {
		case event.TypeRequireUserConfirm, event.TypeRequireExternalExecution:
			return StatusParked
		}
		if c.hasLoggedEvents(ctx, sessionID) {
			return StatusIdle
		}
	}
	if c.sm.HasCompleted(sessionID) {
		return StatusIdle
	}
	return StatusUnknown
}

// lastLoggedEventType walks the session's event log to its tail and returns
// the type of the final entry ("" when absent). Pages of 100 keep the walk
// cheap; 1000 pages is a hard stop against pathological logs.
func (c *SessionCoordinator) lastLoggedEventType(ctx context.Context, sessionID string) string {
	ns := c.keys.SessionEventLogNS(sessionID)
	var cursor int64
	var last string
	for i := 0; i < 1000; i++ {
		entries, next, err := c.cb.LogRead(ctx, ns, cursor, 100)
		if err != nil || len(entries) == 0 {
			return last
		}
		if ev, uerr := event.UnmarshalEvent(entries[len(entries)-1]); uerr == nil {
			last = ev.EventType()
		}
		if next <= cursor || len(entries) < 100 {
			break
		}
		cursor = next
	}
	return last
}

// hasLoggedEvents reports whether the session's event log holds anything.
func (c *SessionCoordinator) hasLoggedEvents(ctx context.Context, sessionID string) bool {
	entries, _, err := c.cb.LogRead(ctx, c.keys.SessionEventLogNS(sessionID), 0, 1)
	return err == nil && len(entries) > 0
}

// --- BgTask registry ---

// BgTask records one background task spawned by a session run (the turn
// itself, tool offloads, and — from 18.5 — long-connection worker jobs).
type BgTask struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"session_id"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"` // running | done | failed
	Err        string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// BgTask statuses.
const (
	BgTaskRunning = "running"
	BgTaskDone    = "done"
	BgTaskFailed  = "failed"
)

// BgTaskRegistry is a concurrency-safe registry of background tasks. The
// registry is per-process; cross-replica visibility of tasks rides on the
// session-run registry entries and lands fully in 18.5.
type BgTaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]*BgTask
}

// NewBgTaskRegistry creates an empty registry.
func NewBgTaskRegistry() *BgTaskRegistry {
	return &BgTaskRegistry{tasks: make(map[string]*BgTask)}
}

// Register records a new running task and returns its id together with a
// done func that marks the task finished (nil error → done, else failed).
func (r *BgTaskRegistry) Register(sessionID, kind string) (string, func(err error)) {
	id := uuid.NewString()
	r.mu.Lock()
	r.tasks[id] = &BgTask{
		ID: id, SessionID: sessionID, Kind: kind,
		Status: BgTaskRunning, StartedAt: time.Now().UTC(),
	}
	r.mu.Unlock()
	done := func(err error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		t, ok := r.tasks[id]
		if !ok {
			return
		}
		now := time.Now().UTC()
		t.FinishedAt = &now
		if err != nil {
			t.Status = BgTaskFailed
			t.Err = err.Error()
		} else {
			t.Status = BgTaskDone
		}
	}
	return id, done
}

// MarkDone finishes a task by id.
func (r *BgTaskRegistry) MarkDone(id string, err error) {
	r.mu.RLock()
	t, ok := r.tasks[id]
	r.mu.RUnlock()
	if !ok {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	t.FinishedAt = &now
	if err != nil {
		t.Status = BgTaskFailed
		t.Err = err.Error()
	} else {
		t.Status = BgTaskDone
	}
}

// List returns every task of a session, newest first.
func (r *BgTaskRegistry) List(sessionID string) []BgTask {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BgTask, 0, len(r.tasks))
	for _, t := range r.tasks {
		if sessionID == "" || t.SessionID == sessionID {
			out = append(out, *t)
		}
	}
	// Newest first for operator-facing lists.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].StartedAt.After(out[j-1].StartedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Get returns one task by id.
func (r *BgTaskRegistry) Get(id string) (BgTask, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[id]
	if !ok {
		return BgTask{}, false
	}
	return *t, true
}
