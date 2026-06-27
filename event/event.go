// Package event provides the V2 fine-grained event-stream protocol for AgentScope.Go.
// It is aligned with AgentScope Python v2's AgentEvent hierarchy to ensure
// cross-language interoperability.
//
// Design principles:
//   - All events carry reply_id for traceability within a single reply_stream() call.
//   - Block-level events carry block_index to disambiguate multiple blocks of the same type.
//   - HITL events use confirm_id for suspend-resume correlation.
//   - JSON field names match Python v2 for Studio UI compatibility.
package event

import (
	"time"
)

// AgentEvent is the root interface for all V2 events.
type AgentEvent interface {
	// EventType returns the concrete event type string (e.g. "reply_start").
	EventType() string

	// Timestamp returns when the event was emitted.
	Timestamp() time.Time

	// ReplyID returns the UUID of the reply stream this event belongs to.
	ReplyID() string
}

// baseEvent provides common fields embedded by all concrete event types.
type baseEvent struct {
	EventType_ string    `json:"event_type"`
	Ts         time.Time `json:"timestamp"`
	ReplyID_   string    `json:"reply_id"`
}

//nolint:revive
func (e baseEvent) EventType() string { return e.EventType_ }

//nolint:revive
func (e baseEvent) Timestamp() time.Time { return e.Ts }

//nolint:revive
func (e baseEvent) ReplyID() string { return e.ReplyID_ }

// Event type constants (aligned with Python v2).
const (
	TypeReplyStart               = "reply_start"
	TypeReplyEnd                 = "reply_end"
	TypeModelCallStart           = "model_call_start"
	TypeModelCallEnd             = "model_call_end"
	TypeTextBlockStart           = "text_block_start"
	TypeTextBlockDelta           = "text_block_delta"
	TypeTextBlockEnd             = "text_block_end"
	TypeDataBlockStart           = "data_block_start"
	TypeDataBlockDelta           = "data_block_delta"
	TypeDataBlockEnd             = "data_block_end"
	TypeThinkingBlockStart       = "thinking_block_start"
	TypeThinkingBlockDelta       = "thinking_block_delta"
	TypeThinkingBlockEnd         = "thinking_block_end"
	TypeHintBlockStart           = "hint_block_start"
	TypeHintBlockDelta           = "hint_block_delta"
	TypeHintBlockEnd             = "hint_block_end"
	TypeToolCallStart            = "tool_call_start"
	TypeToolCallDelta            = "tool_call_delta"
	TypeToolCallEnd              = "tool_call_end"
	TypeRequireUserConfirm       = "require_user_confirm"
	TypeRequireExternalExecution = "require_external_execution"
	TypeToolResultStart          = "tool_result_start"
	TypeToolResultTextDelta      = "tool_result_text_delta"
	TypeToolResultDataDelta      = "tool_result_data_delta"
	TypeToolResultEnd            = "tool_result_end"
	TypeUserConfirmResult        = "user_confirm_result"
	TypeExternalExecutionResult  = "external_execution_result"
	TypeExceedMaxIters           = "exceed_max_iters"
	TypeError                    = "error"
	TypeInterrupt                = "interrupt"
)

// NewBase creates a baseEvent. Used by concrete event constructors.
func NewBase(eventType, replyID string) baseEvent {
	return baseEvent{
		EventType_: eventType,
		Ts:         time.Now(),
		ReplyID_:   replyID,
	}
}

// ConfirmDecision represents a single tool-call decision inside a
// UserConfirmResultEvent.
type ConfirmDecision struct {
	ToolCallID   string         `json:"tool_call_id"`
	Decision     string         `json:"decision"` // "allow" | "always_allow" | "deny" | "modify"
	ModifiedArgs map[string]any `json:"modified_args,omitempty"`
}

// ExternalExecutionResult represents the outcome of an external tool execution.
type ExternalExecutionResult struct {
	ToolCallID string `json:"tool_call_id"`
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}
