package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

// TestReActAgent_ThinkingOnlyContinuesLoop verifies that a response carrying
// only thinking blocks does not finish the turn with an empty message; the loop
// continues until the model produces user-visible content (PyV2 #2120).
func TestReActAgent_ThinkingOnlyContinuesLoop(t *testing.T) {
	thinkingOnly := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewThinkingBlock("still reasoning", ""),
	).Build()
	final := message.NewMsg().Role(message.RoleAssistant).TextContent("answer").Build()

	m := &mockToolModel{name: "m", responses: []*message.Msg{thinkingOnly, final}}
	a, err := Builder().Name("Test").Model(m).Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.GetTextContent(); got != "answer" {
		t.Fatalf("expected loop to continue past thinking-only response, got %q", got)
	}
	if m.calls < 2 {
		t.Fatalf("expected at least 2 model calls, got %d", m.calls)
	}
}

func TestHasOnlyThinkingBlocks(t *testing.T) {
	thinkingOnly := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewThinkingBlock("a", ""),
	).Build()
	if !hasOnlyThinkingBlocks(thinkingOnly) {
		t.Fatal("expected thinking-only message to be detected")
	}

	mixed := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewThinkingBlock("a", ""),
		message.NewTextBlock("answer"),
	).Build()
	if hasOnlyThinkingBlocks(mixed) {
		t.Fatal("mixed thinking+text must not be treated as thinking-only")
	}

	empty := message.NewMsg().Role(message.RoleAssistant).Build()
	if hasOnlyThinkingBlocks(empty) {
		t.Fatal("empty message must not be treated as thinking-only")
	}

	if hasOnlyThinkingBlocks(nil) {
		t.Fatal("nil message must not be treated as thinking-only")
	}
}
