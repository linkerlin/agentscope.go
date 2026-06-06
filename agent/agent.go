package agent

import (
	"context"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// Agent is the core interface for all agent types (backward compatible).
type Agent interface {
	Name() string
	Call(ctx context.Context, msg *message.Msg) (*message.Msg, error)
	CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error)
}

// V2Agent extends Agent with true event-streaming, state management,
// and external event injection for suspend-resume (HITL).
// This is a separate interface so existing Agent implementations do not break.
type V2Agent interface {
	Agent

	// ReplyStream returns a true event stream.
	// Events are fine-grained (block-level deltas, HITL suspend points, etc.)
	// and can be consumed in real time by UIs, A2A peers, or loggers.
	ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error)

	// LoadState restores the agent's runtime state from a snapshot.
	// Used to resume a suspended reply across HTTP requests or after a crash.
	LoadState(state *AgentState) error

	// SaveState captures the agent's current runtime state.
	// Called automatically when the agent suspends for HITL.
	SaveState() (*AgentState, error)

	// InjectEvent allows an external consumer (HTTP handler, WebSocket,
	// another goroutine) to inject a resume event into a suspended agent.
	// Supported events: UserConfirmResultEvent, ExternalExecutionResultEvent.
	InjectEvent(ctx context.Context, ev event.AgentEvent) error
}
