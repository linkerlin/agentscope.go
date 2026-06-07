package event

// ReplyStartEvent marks the beginning of a reply stream.
type ReplyStartEvent struct {
	baseEvent
	AgentName string `json:"agent_name"`
}

// NewReplyStart creates a ReplyStartEvent.
func NewReplyStart(replyID, agentName string) *ReplyStartEvent {
	return &ReplyStartEvent{
		baseEvent: NewBase(TypeReplyStart, replyID),
		AgentName: agentName,
	}
}

// ReplyEndEvent marks the end of a reply stream.
type ReplyEndEvent struct {
	baseEvent
	AgentName string `json:"agent_name"`
}

// NewReplyEnd creates a ReplyEndEvent.
func NewReplyEnd(replyID, agentName string) *ReplyEndEvent {
	return &ReplyEndEvent{
		baseEvent: NewBase(TypeReplyEnd, replyID),
		AgentName: agentName,
	}
}

// ModelCallStartEvent marks the beginning of a model API call.
type ModelCallStartEvent struct {
	baseEvent
	ModelName string `json:"model_name"`
}

// NewModelCallStart creates a ModelCallStartEvent.
func NewModelCallStart(replyID, modelName string) *ModelCallStartEvent {
	return &ModelCallStartEvent{
		baseEvent: NewBase(TypeModelCallStart, replyID),
		ModelName: modelName,
	}
}

// ModelCallEndEvent marks the end of a model API call.
type ModelCallEndEvent struct {
	baseEvent
	ModelName string `json:"model_name"`
}

// NewModelCallEnd creates a ModelCallEndEvent.
func NewModelCallEnd(replyID, modelName string) *ModelCallEndEvent {
	return &ModelCallEndEvent{
		baseEvent: NewBase(TypeModelCallEnd, replyID),
		ModelName: modelName,
	}
}

// ExceedMaxItersEvent signals that the agent reached the maximum
// number of iterations without producing a final answer.
type ExceedMaxItersEvent struct {
	baseEvent
	MaxIters int `json:"max_iters"`
}

// NewExceedMaxIters creates an ExceedMaxItersEvent.
func NewExceedMaxIters(replyID string, maxIters int) *ExceedMaxItersEvent {
	return &ExceedMaxItersEvent{
		baseEvent: NewBase(TypeExceedMaxIters, replyID),
		MaxIters:  maxIters,
	}
}

// ErrorEvent signals an error during reply execution.
type ErrorEvent struct {
	baseEvent
	Err string `json:"error"`
}

// NewError creates an ErrorEvent.
func NewError(replyID string, err error) *ErrorEvent {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &ErrorEvent{
		baseEvent: NewBase(TypeError, replyID),
		Err:       msg,
	}
}

// InterruptEvent signals that the agent was interrupted.
type InterruptEvent struct {
	baseEvent
	Source string `json:"source"`
}

// NewInterrupt creates an InterruptEvent.
func NewInterrupt(replyID, source string) *InterruptEvent {
	return &InterruptEvent{
		baseEvent: NewBase(TypeInterrupt, replyID),
		Source:    source,
	}
}

// Ensure interface compliance at compile time.
var (
	_ AgentEvent = (*ReplyStartEvent)(nil)
	_ AgentEvent = (*ReplyEndEvent)(nil)
	_ AgentEvent = (*ModelCallStartEvent)(nil)
	_ AgentEvent = (*ModelCallEndEvent)(nil)
	_ AgentEvent = (*ExceedMaxItersEvent)(nil)
	_ AgentEvent = (*ErrorEvent)(nil)
	_ AgentEvent = (*InterruptEvent)(nil)
)
