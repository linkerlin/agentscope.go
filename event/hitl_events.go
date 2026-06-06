package event

// RequireUserConfirmEvent signals that the agent is suspended and waiting
// for user confirmation (or denial / modification) of pending tool calls.
// This is a suspend-point in the event stream.
type RequireUserConfirmEvent struct {
	baseEvent
	ConfirmID string `json:"confirm_id"`
	// ToolCalls holds the tool calls pending confirmation.
	ToolCalls []ToolCallSummary `json:"tool_calls"`
}

// ToolCallSummary is a lightweight summary of a tool call for confirmation UI.
type ToolCallSummary struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
}

// NewRequireUserConfirm creates a RequireUserConfirmEvent.
func NewRequireUserConfirm(replyID, confirmID string, calls []ToolCallSummary) *RequireUserConfirmEvent {
	return &RequireUserConfirmEvent{
		baseEvent: NewBase(TypeRequireUserConfirm, replyID),
		ConfirmID: confirmID,
		ToolCalls: calls,
	}
}

// RequireExternalExecutionEvent signals that the agent is suspended and
// waiting for an external system to execute tool calls.
type RequireExternalExecutionEvent struct {
	baseEvent
	ConfirmID string            `json:"confirm_id"`
	ToolCalls []ToolCallSummary `json:"tool_calls"`
}

// NewRequireExternalExecution creates a RequireExternalExecutionEvent.
func NewRequireExternalExecution(replyID, confirmID string, calls []ToolCallSummary) *RequireExternalExecutionEvent {
	return &RequireExternalExecutionEvent{
		baseEvent: NewBase(TypeRequireExternalExecution, replyID),
		ConfirmID: confirmID,
		ToolCalls: calls,
	}
}

// UserConfirmResultEvent is injected by an external consumer to resume the
// agent after a RequireUserConfirmEvent.
type UserConfirmResultEvent struct {
	baseEvent
	ConfirmID string            `json:"confirm_id"`
	Decisions []ConfirmDecision `json:"decisions"`
}

// NewUserConfirmResult creates a UserConfirmResultEvent.
func NewUserConfirmResult(replyID, confirmID string, decisions []ConfirmDecision) *UserConfirmResultEvent {
	return &UserConfirmResultEvent{
		baseEvent: NewBase(TypeUserConfirmResult, replyID),
		ConfirmID: confirmID,
		Decisions: decisions,
	}
}

// ExternalExecutionResultEvent is injected by an external system to resume
// the agent after a RequireExternalExecutionEvent.
type ExternalExecutionResultEvent struct {
	baseEvent
	ConfirmID string                  `json:"confirm_id"`
	Results   []ExternalExecutionResult `json:"results"`
}

// NewExternalExecutionResult creates an ExternalExecutionResultEvent.
func NewExternalExecutionResult(replyID, confirmID string, results []ExternalExecutionResult) *ExternalExecutionResultEvent {
	return &ExternalExecutionResultEvent{
		baseEvent: NewBase(TypeExternalExecutionResult, replyID),
		ConfirmID: confirmID,
		Results:   results,
	}
}

// Ensure interface compliance at compile time.
var (
	_ AgentEvent = (*RequireUserConfirmEvent)(nil)
	_ AgentEvent = (*RequireExternalExecutionEvent)(nil)
	_ AgentEvent = (*UserConfirmResultEvent)(nil)
	_ AgentEvent = (*ExternalExecutionResultEvent)(nil)
)
