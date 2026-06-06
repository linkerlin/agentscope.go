package event

// TextBlockStartEvent signals the start of a new text content block.
type TextBlockStartEvent struct {
	baseEvent
	BlockIndex int `json:"block_index"`
}

// NewTextBlockStart creates a TextBlockStartEvent.
func NewTextBlockStart(replyID string, blockIndex int) *TextBlockStartEvent {
	return &TextBlockStartEvent{
		baseEvent:  NewBase(TypeTextBlockStart, replyID),
		BlockIndex: blockIndex,
	}
}

// TextBlockDeltaEvent carries an incremental text fragment.
type TextBlockDeltaEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	Delta      string `json:"delta"`
}

// NewTextBlockDelta creates a TextBlockDeltaEvent.
func NewTextBlockDelta(replyID string, blockIndex int, delta string) *TextBlockDeltaEvent {
	return &TextBlockDeltaEvent{
		baseEvent:  NewBase(TypeTextBlockDelta, replyID),
		BlockIndex: blockIndex,
		Delta:      delta,
	}
}

// TextBlockEndEvent signals the end of a text content block.
type TextBlockEndEvent struct {
	baseEvent
	BlockIndex int `json:"block_index"`
}

// NewTextBlockEnd creates a TextBlockEndEvent.
func NewTextBlockEnd(replyID string, blockIndex int) *TextBlockEndEvent {
	return &TextBlockEndEvent{
		baseEvent:  NewBase(TypeTextBlockEnd, replyID),
		BlockIndex: blockIndex,
	}
}

// ThinkingBlockStartEvent signals the start of a reasoning/thinking block.
type ThinkingBlockStartEvent struct {
	baseEvent
	BlockIndex int `json:"block_index"`
}

// NewThinkingBlockStart creates a ThinkingBlockStartEvent.
func NewThinkingBlockStart(replyID string, blockIndex int) *ThinkingBlockStartEvent {
	return &ThinkingBlockStartEvent{
		baseEvent:  NewBase(TypeThinkingBlockStart, replyID),
		BlockIndex: blockIndex,
	}
}

// ThinkingBlockDeltaEvent carries an incremental reasoning fragment.
type ThinkingBlockDeltaEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	Delta      string `json:"delta"`
}

// NewThinkingBlockDelta creates a ThinkingBlockDeltaEvent.
func NewThinkingBlockDelta(replyID string, blockIndex int, delta string) *ThinkingBlockDeltaEvent {
	return &ThinkingBlockDeltaEvent{
		baseEvent:  NewBase(TypeThinkingBlockDelta, replyID),
		BlockIndex: blockIndex,
		Delta:      delta,
	}
}

// ThinkingBlockEndEvent signals the end of a thinking block.
type ThinkingBlockEndEvent struct {
	baseEvent
	BlockIndex int `json:"block_index"`
}

// NewThinkingBlockEnd creates a ThinkingBlockEndEvent.
func NewThinkingBlockEnd(replyID string, blockIndex int) *ThinkingBlockEndEvent {
	return &ThinkingBlockEndEvent{
		baseEvent:  NewBase(TypeThinkingBlockEnd, replyID),
		BlockIndex: blockIndex,
	}
}

// ToolCallStartEvent signals the start of a tool call block.
type ToolCallStartEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
}

// NewToolCallStart creates a ToolCallStartEvent.
func NewToolCallStart(replyID string, blockIndex int, toolCallID, toolName string) *ToolCallStartEvent {
	return &ToolCallStartEvent{
		baseEvent:  NewBase(TypeToolCallStart, replyID),
		BlockIndex: blockIndex,
		ToolCallID: toolCallID,
		ToolName:   toolName,
	}
}

// ToolCallDeltaEvent carries an incremental tool-call argument fragment
// (used for streaming JSON parsing of tool arguments).
type ToolCallDeltaEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	ToolCallID string `json:"tool_call_id"`
	Delta      string `json:"delta"`
}

// NewToolCallDelta creates a ToolCallDeltaEvent.
func NewToolCallDelta(replyID string, blockIndex int, toolCallID, delta string) *ToolCallDeltaEvent {
	return &ToolCallDeltaEvent{
		baseEvent:  NewBase(TypeToolCallDelta, replyID),
		BlockIndex: blockIndex,
		ToolCallID: toolCallID,
		Delta:      delta,
	}
}

// ToolCallEndEvent signals the end of a tool call block.
type ToolCallEndEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	ToolCallID string `json:"tool_call_id"`
}

// NewToolCallEnd creates a ToolCallEndEvent.
func NewToolCallEnd(replyID string, blockIndex int, toolCallID string) *ToolCallEndEvent {
	return &ToolCallEndEvent{
		baseEvent:  NewBase(TypeToolCallEnd, replyID),
		BlockIndex: blockIndex,
		ToolCallID: toolCallID,
	}
}

// ToolResultStartEvent signals the start of a tool result block.
type ToolResultStartEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
}

// NewToolResultStart creates a ToolResultStartEvent.
func NewToolResultStart(replyID string, blockIndex int, toolCallID, toolName string) *ToolResultStartEvent {
	return &ToolResultStartEvent{
		baseEvent:  NewBase(TypeToolResultStart, replyID),
		BlockIndex: blockIndex,
		ToolCallID: toolCallID,
		ToolName:   toolName,
	}
}

// ToolResultTextDeltaEvent carries an incremental text fragment of a tool result.
type ToolResultTextDeltaEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	ToolCallID string `json:"tool_call_id"`
	Delta      string `json:"delta"`
}

// NewToolResultTextDelta creates a ToolResultTextDeltaEvent.
func NewToolResultTextDelta(replyID string, blockIndex int, toolCallID, delta string) *ToolResultTextDeltaEvent {
	return &ToolResultTextDeltaEvent{
		baseEvent:  NewBase(TypeToolResultTextDelta, replyID),
		BlockIndex: blockIndex,
		ToolCallID: toolCallID,
		Delta:      delta,
	}
}

// ToolResultEndEvent signals the end of a tool result block.
type ToolResultEndEvent struct {
	baseEvent
	BlockIndex int    `json:"block_index"`
	ToolCallID string `json:"tool_call_id"`
}

// NewToolResultEnd creates a ToolResultEndEvent.
func NewToolResultEnd(replyID string, blockIndex int, toolCallID string) *ToolResultEndEvent {
	return &ToolResultEndEvent{
		baseEvent:  NewBase(TypeToolResultEnd, replyID),
		BlockIndex: blockIndex,
		ToolCallID: toolCallID,
	}
}

// Ensure interface compliance at compile time.
var (
	_ AgentEvent = (*TextBlockStartEvent)(nil)
	_ AgentEvent = (*TextBlockDeltaEvent)(nil)
	_ AgentEvent = (*TextBlockEndEvent)(nil)
	_ AgentEvent = (*ThinkingBlockStartEvent)(nil)
	_ AgentEvent = (*ThinkingBlockDeltaEvent)(nil)
	_ AgentEvent = (*ThinkingBlockEndEvent)(nil)
	_ AgentEvent = (*ToolCallStartEvent)(nil)
	_ AgentEvent = (*ToolCallDeltaEvent)(nil)
	_ AgentEvent = (*ToolCallEndEvent)(nil)
	_ AgentEvent = (*ToolResultStartEvent)(nil)
	_ AgentEvent = (*ToolResultTextDeltaEvent)(nil)
	_ AgentEvent = (*ToolResultEndEvent)(nil)
)
