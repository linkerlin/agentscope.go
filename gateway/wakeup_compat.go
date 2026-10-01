// Package gateway — wakeup_compat.go keeps the pre-16.2 public surface of
// the wakeup dispatcher while the implementation lives in gateway/wakeupapi.
// WakeupDispatcher is a pure type alias (same type, promoted methods); the
// constructor is a thin wrapper that narrows *SessionManager to the
// wakeupapi.Sessions interface with an explicit typed-nil guard.
package gateway

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/gateway/wakeupapi"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// sessionRunFunc starts a session turn, usually through the coordinator
// (cross-replica single-consumer semantics, 18.5). nil means "use the
// in-process SessionManager". It is the gateway-root twin of
// wakeupapi.RunFunc / scheduleapi.RunFunc (identical signatures).
type sessionRunFunc func(ctx context.Context, sessionID string, a agent.Agent, msg *message.Msg) (<-chan event.AgentEvent, error)

// WakeupDispatcher is implemented in gateway/wakeupapi (16.2).
type WakeupDispatcher = wakeupapi.WakeupDispatcher

// NewWakeupDispatcher creates a dispatcher backed by this server's session
// manager. buildAgent should resolve to Server.buildSessionAgentFromStorage
// (or an equivalent per-session builder).
func NewWakeupDispatcher(
	bus messagebus.TeamBus,
	sm *SessionManager,
	storage service.Storage,
	buildAgent func(ctx context.Context, agentID, sessionID string) (agent.Agent, error),
) *WakeupDispatcher {
	var sessions wakeupapi.Sessions
	if sm != nil {
		sessions = sm // typed-nil guard: a nil *SessionManager must not enter the interface
	}
	return wakeupapi.NewWakeupDispatcher(bus, sessions, storage, buildAgent)
}

// WithWakeupRun adapts the root's sessionRunFunc to wakeupapi.RunFunc
// (identical underlying signatures, distinct named types).
func WithWakeupRun(fn sessionRunFunc) wakeupapi.RunFunc { return wakeupapi.RunFunc(fn) }
