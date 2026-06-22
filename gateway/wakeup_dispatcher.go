// Package gateway — wakeup_dispatcher.go realises Python agentscope's
// WakeupDispatcher: a single background loop subscribes to the bus's wakeup
// signal stream and, for each idle session, drains its inbox of pending
// <team-message> blocks and re-runs the agent with them as input. Busy
// sessions are retried until idle (messages stay persisted in the inbox).
package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// wakeupBusyTimeout caps how long a busy-session retry waits before giving up
// (leaving messages in the inbox for a future wakeup).
const wakeupBusyTimeout = 30 * time.Second

// wakeupPollInterval is the poll cadence while waiting for a busy session.
const wakeupPollInterval = 200 * time.Millisecond

// WakeupDispatcher drains team inboxes and re-runs idle worker sessions. It is
// the async collaboration engine: TeamSay/AgentCreate enqueue wakeups; this
// loop turns them into actual agent runs. Mirrors Python agentscope's
// app/_manager/_wakeup_dispatcher.py.
type WakeupDispatcher struct {
	bus        messagebus.TeamBus
	sessionMgr *SessionManager
	storage    service.Storage
	// buildAgent constructs a fresh agent for a session (Server.buildSessionAgentFromStorage).
	buildAgent func(ctx context.Context, agentID, sessionID string) (agent.Agent, error)

	cancel context.CancelFunc
	done   chan struct{}
}

// NewWakeupDispatcher creates a dispatcher. buildAgent should resolve to
// Server.buildSessionAgentFromStorage (or an equivalent per-session builder).
func NewWakeupDispatcher(
	bus messagebus.TeamBus,
	sm *SessionManager,
	storage service.Storage,
	buildAgent func(ctx context.Context, agentID, sessionID string) (agent.Agent, error),
) *WakeupDispatcher {
	return &WakeupDispatcher{
		bus:        bus,
		sessionMgr: sm,
		storage:    storage,
		buildAgent: buildAgent,
	}
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
	if d.sessionMgr != nil && d.sessionMgr.IsActive(sessionID) {
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
			if d.sessionMgr == nil || !d.sessionMgr.IsActive(sessionID) {
				d.drainAndRun(ctx, sessionID)
				return
			}
		}
	}
	// Timed out: leave messages in the inbox for a future wakeup.
}

// drainAndRun is the core: read inbox, build the agent, assemble team messages,
// and kick off a run. Errors are non-fatal (logged via response text only).
func (d *WakeupDispatcher) drainAndRun(ctx context.Context, sessionID string) {
	// Orphan guard: session must still exist.
	se, err := d.storage.GetSession(ctx, sessionID)
	if err != nil || se == nil {
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
		return
	}
	// Assemble inbox messages into a single user turn.
	var sb strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&sb, "<team-message from=%q>\n%s\n</team-message>\n\n", m.From, m.Content)
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent(sb.String()).Build()
	// Fire and forget: SessionManager serialises per-session and persists the reply.
	_, _ = d.sessionMgr.Run(ctx, sessionID, ag, msg)
}
