package event

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	replyID := "reply-123"

	cases := []struct {
		name  string
		event AgentEvent
	}{
		{"reply_start", NewReplyStart(replyID, "test-agent")},
		{"reply_end", NewReplyEnd(replyID, "test-agent")},
		{"text_block_delta", NewTextBlockDelta(replyID, 0, "hello")},
		{"thinking_block_delta", NewThinkingBlockDelta(replyID, 1, "reasoning...")},
		{"tool_call_start", NewToolCallStart(replyID, 0, "tc1", "calculator")},
		{"tool_call_delta", NewToolCallDelta(replyID, 0, "tc1", `{"a":`)},
		{"tool_call_end", NewToolCallEnd(replyID, 0, "tc1")},
		{"tool_result_text_delta", NewToolResultTextDelta(replyID, 0, "tc1", "42")},
		{"require_user_confirm", NewRequireUserConfirm(replyID, "cfm-1", []ToolCallSummary{
			{ID: "tc1", Name: "calculator", Input: map[string]any{"expr": "1+1"}},
		})},
		{"user_confirm_result", NewUserConfirmResult(replyID, "cfm-1", []ConfirmDecision{
			{ToolCallID: "tc1", Decision: "allow"},
		})},
		{"external_execution_result", NewExternalExecutionResult(replyID, "cfm-2", []ExternalExecutionResult{
			{ToolCallID: "tc1", Success: true, Output: "42"},
		})},
		{"error", NewError(replyID, errors.New("something went wrong"))},
		{"interrupt", NewInterrupt(replyID, "user")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := MarshalEvent(tc.event)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			// Verify event_type is present in raw JSON.
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal raw: %v", err)
			}
			if raw["event_type"] == nil || raw["event_type"] == "" {
				t.Fatalf("missing event_type in JSON: %s", string(data))
			}
			if raw["reply_id"] != replyID {
				t.Fatalf("reply_id mismatch: got %v", raw["reply_id"])
			}

			// Round-trip through UnmarshalEvent.
			restored, err := UnmarshalEvent(data)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if restored.EventType() != tc.event.EventType() {
				t.Fatalf("event_type mismatch: want %s got %s", tc.event.EventType(), restored.EventType())
			}
			if restored.ReplyID() != tc.event.ReplyID() {
				t.Fatalf("reply_id mismatch: want %s got %s", tc.event.ReplyID(), restored.ReplyID())
			}
		})
	}
}

func TestUnmarshalUnknownType(t *testing.T) {
	data := []byte(`{"event_type":"unknown_type","reply_id":"r1","timestamp":"2026-01-01T00:00:00Z"}`)
	_, err := UnmarshalEvent(data)
	if err == nil {
		t.Fatal("expected error for unknown event_type")
	}
}

func TestUnmarshalMissingType(t *testing.T) {
	data := []byte(`{"reply_id":"r1","timestamp":"2026-01-01T00:00:00Z"}`)
	_, err := UnmarshalEvent(data)
	if err == nil {
		t.Fatal("expected error for missing event_type")
	}
}

func TestTimestampPreserved(t *testing.T) {
	replyID := "reply-ts"
	e := NewTextBlockDelta(replyID, 0, "x")
	data, _ := MarshalEvent(e)

	restored, err := UnmarshalEvent(data)
	if err != nil {
		t.Fatal(err)
	}

	// Allow small delta because of JSON time precision.
	diff := restored.Timestamp().Sub(e.Timestamp())
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Fatalf("timestamp drift too large: %v", diff)
	}
}
