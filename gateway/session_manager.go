package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

// SessionManager manages in-flight agent runs per session.
//
// Responsibilities:
//   - Serialisation: at most one active run per session_id at a time;
//     additional callers block on a per-session mutex and run in order.
//   - Fan-out buffer: every event produced during a run is appended to a
//     replay buffer AND pushed to all active subscriber channels so that
//     clients joining mid-run receive the full event history.
//   - Completed-run replay: when a run finishes its final buffer is kept
//     briefly so that clients connecting immediately after completion can
//     still receive the full result.
//   - Lifecycle: the buffer and subscriber list are created when a run
//     starts and discarded when it ends, keeping memory bounded.
type SessionManager struct {
	locks     map[string]*sync.Mutex        // session_id -> serialisation lock
	runs      map[string]*sessionRun        // session_id -> in-flight run state
	completed map[string][]event.AgentEvent // session_id -> final buffer after run ends
	mu        sync.RWMutex
	storage   service.Storage // optional persistence layer for Msg upsert
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		locks:     make(map[string]*sync.Mutex),
		runs:      make(map[string]*sessionRun),
		completed: make(map[string][]event.AgentEvent),
	}
}

// WithStorage attaches a Storage for automatic Msg persistence.
func (sm *SessionManager) WithStorage(st service.Storage) *SessionManager {
	sm.storage = st
	return sm
}

// sessionRun holds the state for a single in-flight agent run.
type sessionRun struct {
	replyID     string
	buffer      []event.AgentEvent      // all events produced so far
	subscribers []chan event.AgentEvent // active subscriber channels
	agent       agent.Agent
	cancel      context.CancelFunc
	mu          sync.RWMutex
	done        bool
	terminating bool
}

// Run executes an agent reply for the given session and returns a channel
// that receives all events (including replay for late-joining subscribers).
//
// If a run is already active for this session, the caller blocks until the
// previous run completes, then starts a new run.
//
// When storage is configured, Run automatically persists the input message
// (via UpsertMessage) and the reconstructed assistant reply message
// (assembled via Msg.AppendEvent from the event stream).
func (sm *SessionManager) Run(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error) {
	lock := sm.getLock(sessionID)
	lock.Lock()

	v2, ok := a.(agent.V2Agent)
	if !ok {
		lock.Unlock()
		return nil, fmt.Errorf("session_manager: agent does not support V2 streaming")
	}

	// Persist input message before starting the stream.
	if sm.storage != nil && msg != nil && sessionID != "" {
		storedMsg := service.MsgToStored(msg, sessionID)
		_ = sm.storage.UpsertMessage(ctx, storedMsg)
	}

	runCtx, cancel := context.WithCancel(ContextWithSessionID(ctx, sessionID))
	ch, err := v2.ReplyStream(runCtx, msg)
	if err != nil {
		cancel()
		lock.Unlock()
		return nil, fmt.Errorf("session_manager: reply stream error: %w", err)
	}

	run := &sessionRun{
		buffer:      make([]event.AgentEvent, 0, 64),
		subscribers: make([]chan event.AgentEvent, 0, 4),
		agent:       a,
		cancel:      cancel,
	}

	sm.mu.Lock()
	sm.runs[sessionID] = run
	sm.mu.Unlock()

	// Create the first subscriber channel for this caller.
	sub := make(chan event.AgentEvent, 64)
	run.mu.Lock()
	run.subscribers = append(run.subscribers, sub)
	run.mu.Unlock()

	// Fan-out goroutine: consumes ReplyStream, distributes to all subscribers,
	// and incrementally builds the reply Msg for persistence.
	go func() {
		replyMsg := message.NewMsg().Role(message.RoleAssistant).Build()

		defer cancel()
		defer lock.Unlock()
		defer func() {
			run.mu.Lock()
			subs := make([]chan event.AgentEvent, len(run.subscribers))
			copy(subs, run.subscribers)
			finalBuf := make([]event.AgentEvent, len(run.buffer))
			copy(finalBuf, run.buffer)
			run.done = true
			run.mu.Unlock()

			// Persist the reconstructed reply message before closing subscribers.
			if sm.storage != nil && sessionID != "" {
				storedMsg := service.MsgToStored(replyMsg, sessionID)
				_ = sm.storage.UpsertMessage(ctx, storedMsg)
			}

			for _, s := range subs {
				close(s)
			}
			sm.mu.Lock()
			delete(sm.runs, sessionID)
			sm.completed[sessionID] = finalBuf
			sm.mu.Unlock()
		}()

		for ev := range ch {
			if ev == nil {
				continue
			}
			replyMsg.AppendEvent(ev)
			run.mu.Lock()
			run.buffer = append(run.buffer, ev)
			subs := make([]chan event.AgentEvent, len(run.subscribers))
			copy(subs, run.subscribers)
			run.mu.Unlock()

			for _, s := range subs {
				select {
				case s <- ev:
				case <-ctx.Done():
				}
			}
		}
	}()

	return sub, nil
}

// Subscribe joins an active run for the given session.
// It returns a channel that first replays all buffered events, then
// continues to receive new events in real-time.
// If the run has already finished, the full final buffer is replayed.
// If no run has ever started for the session, it returns a closed channel.
func (sm *SessionManager) Subscribe(sessionID string) <-chan event.AgentEvent {
	sm.mu.RLock()
	run, active := sm.runs[sessionID]
	completed, hasCompleted := sm.completed[sessionID]
	sm.mu.RUnlock()

	sub := make(chan event.AgentEvent, 64)

	// Case 1: no active run, but we have a completed buffer -> replay it.
	if !active && hasCompleted {
		go func() {
			for _, ev := range completed {
				sub <- ev
			}
			close(sub)
		}()
		return sub
	}

	// Case 2: no active run and no completed buffer -> empty closed channel.
	if !active || run == nil {
		close(sub)
		return sub
	}

	run.mu.RLock()
	// Copy buffer and check done status while holding read lock.
	buf := make([]event.AgentEvent, len(run.buffer))
	copy(buf, run.buffer)
	done := run.done
	if !done {
		run.subscribers = append(run.subscribers, sub)
	}
	run.mu.RUnlock()

	if done {
		// Run already finished inside the critical section (rare race):
		// replay buffer and close.
		go func() {
			for _, ev := range buf {
				sub <- ev
			}
			close(sub)
		}()
		return sub
	}

	// Active run: replay buffer in background, then fan-out goroutine
	// will continue sending new events to this subscriber.
	go func() {
		for _, ev := range buf {
			sub <- ev
		}
	}()

	return sub
}

// Terminate cancels an in-flight run for the session (Streamable HTTP DELETE).
// It cancels the run context and signals Interrupt on the agent when supported.
// Returns true when an active run was found and termination was requested.
func (sm *SessionManager) Terminate(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	sm.mu.RLock()
	run, ok := sm.runs[sessionID]
	sm.mu.RUnlock()
	if !ok || run == nil {
		return false
	}

	run.mu.Lock()
	if run.done || run.terminating {
		run.mu.Unlock()
		return false
	}
	run.terminating = true
	cancel := run.cancel
	ag := run.agent
	run.mu.Unlock()

	interruptAgent(ag)
	if cancel != nil {
		cancel()
	}
	return true
}

// ClearCompleted removes the replay buffer for a finished session run.
func (sm *SessionManager) ClearCompleted(sessionID string) {
	if sessionID == "" {
		return
	}
	sm.mu.Lock()
	delete(sm.completed, sessionID)
	sm.mu.Unlock()
}

func interruptAgent(a agent.Agent) {
	if a == nil {
		return
	}
	if ir, ok := a.(interface{ Interrupt() }); ok {
		ir.Interrupt()
	}
}

// IsActive returns true if the given session has an in-flight run.
func (sm *SessionManager) IsActive(sessionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	run, ok := sm.runs[sessionID]
	if !ok || run == nil {
		return false
	}
	run.mu.RLock()
	defer run.mu.RUnlock()
	return !run.done
}

// ActiveCount returns the number of currently active sessions.
func (sm *SessionManager) ActiveCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	count := 0
	for _, run := range sm.runs {
		run.mu.RLock()
		if !run.done {
			count++
		}
		run.mu.RUnlock()
	}
	return count
}

// getLock returns (creating if necessary) the per-session serialisation lock.
func (sm *SessionManager) getLock(sessionID string) *sync.Mutex {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if m, ok := sm.locks[sessionID]; ok {
		return m
	}
	m := &sync.Mutex{}
	sm.locks[sessionID] = m
	return m
}
