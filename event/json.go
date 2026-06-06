package event

import (
	"encoding/json"
	"fmt"
	"time"
)

// rawEvent is used for deserialization to peek at event_type before
// creating the concrete struct.
type rawEvent struct {
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	ReplyID   string          `json:"reply_id"`
	Payload   json.RawMessage `json:"-"`
}

// MarshalEvent serializes an AgentEvent to JSON.
func MarshalEvent(ev AgentEvent) ([]byte, error) {
	return json.Marshal(ev)
}

// UnmarshalEvent deserializes JSON into the concrete AgentEvent type.
func UnmarshalEvent(data []byte) (AgentEvent, error) {
	// First unmarshal into a map to read event_type.
	var peek map[string]json.RawMessage
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("event: unmarshal peek: %w", err)
	}

	rawType, ok := peek["event_type"]
	if !ok {
		return nil, fmt.Errorf("event: missing event_type field")
	}

	var eventType string
	if err := json.Unmarshal(rawType, &eventType); err != nil {
		return nil, fmt.Errorf("event: unmarshal event_type: %w", err)
	}

	// Unmarshal into concrete type based on event_type.
	switch eventType {
	case TypeReplyStart:
		var e ReplyStartEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeReplyEnd:
		var e ReplyEndEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeTextBlockStart:
		var e TextBlockStartEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeTextBlockDelta:
		var e TextBlockDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeTextBlockEnd:
		var e TextBlockEndEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeThinkingBlockStart:
		var e ThinkingBlockStartEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeThinkingBlockDelta:
		var e ThinkingBlockDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeThinkingBlockEnd:
		var e ThinkingBlockEndEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeToolCallStart:
		var e ToolCallStartEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeToolCallDelta:
		var e ToolCallDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeToolCallEnd:
		var e ToolCallEndEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeRequireUserConfirm:
		var e RequireUserConfirmEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeRequireExternalExecution:
		var e RequireExternalExecutionEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeToolResultStart:
		var e ToolResultStartEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeToolResultTextDelta:
		var e ToolResultTextDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeToolResultEnd:
		var e ToolResultEndEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeUserConfirmResult:
		var e UserConfirmResultEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeExternalExecutionResult:
		var e ExternalExecutionResultEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeError:
		var e ErrorEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case TypeInterrupt:
		var e InterruptEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("event: unknown event_type %q", eventType)
	}
}
