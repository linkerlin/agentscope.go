package gateway

import (
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/event"
)

func TestDefaultAGUIConverter_ReplyLifecycle(t *testing.T) {
	conv := NewDefaultAGUIConverter()
	opts := AGUIConvertOptions{ThreadID: "sess_1"}

	start, err := conv.Convert(event.NewReplyStart("reply_1", "agent"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if start["type"] != "RUN_STARTED" || start["threadId"] != "sess_1" || start["runId"] != "reply_1" {
		t.Fatalf("unexpected RUN_STARTED: %#v", start)
	}

	end, err := conv.Convert(event.NewReplyEnd("reply_1", "agent"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if end["type"] != "RUN_FINISHED" {
		t.Fatalf("unexpected RUN_FINISHED: %#v", end)
	}
}

func TestDefaultAGUIConverter_TextBlock(t *testing.T) {
	conv := NewDefaultAGUIConverter()
	opts := AGUIConvertOptions{}

	delta, err := conv.Convert(event.NewTextBlockDelta("r1", 0, "hello"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if delta["type"] != "TEXT_MESSAGE_CONTENT" || delta["delta"] != "hello" {
		t.Fatalf("unexpected delta: %#v", delta)
	}
}

func TestDefaultAGUIConverter_ToolResult(t *testing.T) {
	conv := NewDefaultAGUIConverter()
	opts := AGUIConvertOptions{}

	_, _ = conv.Convert(event.NewToolResultTextDelta("r1", 0, "tc1", "part1"), opts)
	_, _ = conv.Convert(event.NewToolResultTextDelta("r1", 0, "tc1", "part2"), opts)
	end, err := conv.Convert(event.NewToolResultEnd("r1", 0, "tc1"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if end["type"] != "TOOL_CALL_RESULT" || end["content"] != "part1part2" {
		t.Fatalf("unexpected tool result: %#v", end)
	}
}

func TestEncodeStreamEvent_AGUI(t *testing.T) {
	ev := event.NewTextBlockDelta("r1", 0, "hi")
	data, err := EncodeStreamEvent(ev, AGUIConvertOptions{ThreadID: "s1"}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty payload")
	}
}

func TestEncodeStreamEvent_AGUI_ReusesConverter(t *testing.T) {
	conv := NewDefaultAGUIConverter()
	opts := AGUIConvertOptions{ThreadID: "s1"}
	_, _ = EncodeStreamEvent(event.NewToolResultTextDelta("r1", 0, "tc1", "a"), opts, true, conv)
	_, _ = EncodeStreamEvent(event.NewToolResultTextDelta("r1", 0, "tc1", "b"), opts, true, conv)
	data, err := EncodeStreamEvent(event.NewToolResultEnd("r1", 0, "tc1"), opts, true, conv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"content":"ab"`) {
		t.Fatalf("expected aggregated tool result, got %s", data)
	}
}
