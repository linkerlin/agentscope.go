package agent

import (
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

func TestCollectMessage_BasicText(t *testing.T) {
	ch := make(chan event.AgentEvent, 4)
	ch <- event.NewReplyStart("r1", "mock")
	ch <- event.NewTextBlockDelta("r1", 0, "hello")
	ch <- event.NewTextBlockDelta("r1", 0, " world")
	ch <- event.NewReplyEnd("r1", "mock")
	close(ch)

	msg, err := CollectMessage(ch)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "hello world" {
		t.Fatalf("expected 'hello world', got %q", msg.GetTextContent())
	}
	if msg.Role != message.RoleAssistant {
		t.Fatalf("expected assistant role, got %s", msg.Role)
	}
}

func TestCollectMessage_WithThinking(t *testing.T) {
	ch := make(chan event.AgentEvent, 6)
	ch <- event.NewReplyStart("r1", "mock")
	ch <- event.NewThinkingBlockStart("r1", 0)
	ch <- event.NewThinkingBlockDelta("r1", 0, "think")
	ch <- event.NewThinkingBlockEnd("r1", 0)
	ch <- event.NewTextBlockDelta("r1", 0, "answer")
	ch <- event.NewReplyEnd("r1", "mock")
	close(ch)

	msg, err := CollectMessage(ch)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "answer" {
		t.Fatalf("expected 'answer', got %q", msg.GetTextContent())
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content blocks (text + thinking), got %d", len(msg.Content))
	}
}

func TestCollectMessage_ErrorEvent(t *testing.T) {
	ch := make(chan event.AgentEvent, 2)
	ch <- event.NewReplyStart("r1", "mock")
	ch <- event.NewError("r1", errors.New("boom"))
	close(ch)

	_, err := CollectMessage(ch)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestCollectMessage_EmptyStream(t *testing.T) {
	ch := make(chan event.AgentEvent)
	close(ch)

	msg, err := CollectMessage(ch)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "" {
		t.Fatalf("expected empty text, got %q", msg.GetTextContent())
	}
}

func TestCollectMessage_ToolCall(t *testing.T) {
	ch := make(chan event.AgentEvent, 6)
	ch <- event.NewReplyStart("r1", "mock")
	ch <- event.NewToolCallStart("r1", 0, "tc1", "calc")
	ch <- event.NewToolCallDelta("r1", 0, "tc1", `{"expr":"1+1"}`)
	ch <- event.NewToolCallEnd("r1", 0, "tc1")
	ch <- event.NewReplyEnd("r1", "mock")
	close(ch)

	msg, err := CollectMessage(ch)
	if err != nil {
		t.Fatal(err)
	}
	calls := msg.GetToolUseCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "calc" {
		t.Fatalf("unexpected tool name: %s", calls[0].Name)
	}
}

func TestCollectMessage_ToolResultWithData(t *testing.T) {
	ch := make(chan event.AgentEvent, 6)
	ch <- event.NewReplyStart("r1", "mock")
	ch <- event.NewToolResultStart("r1", 0, "tc1", "image_gen")
	ch <- event.NewToolResultDataDelta("r1", 0, "tc1", "base64data", "image/png")
	ch <- event.NewToolResultEnd("r1", 0, "tc1")
	ch <- event.NewReplyEnd("r1", "mock")
	close(ch)

	msg, err := CollectMessage(ch)
	if err != nil {
		t.Fatal(err)
	}
	results := msg.GetToolResults()
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if results[0].State != "completed" {
		t.Fatalf("expected state completed, got %s", results[0].State)
	}
}

func TestCollectMessage_SetsFinishedAt(t *testing.T) {
	ch := make(chan event.AgentEvent, 3)
	ch <- event.NewReplyStart("r1", "mock")
	ch <- event.NewTextBlockDelta("r1", 0, "done")
	ch <- event.NewReplyEnd("r1", "mock")
	close(ch)

	msg, err := CollectMessage(ch)
	if err != nil {
		t.Fatal(err)
	}
	if msg.FinishedAt == nil {
		t.Fatal("expected FinishedAt to be set by ReplyEndEvent")
	}
}
