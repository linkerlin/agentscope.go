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
	_ AgentEvent = (*ErrorEvent)(nil)
	_ AgentEvent = (*InterruptEvent)(nil)
)
