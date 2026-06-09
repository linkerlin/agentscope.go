package formatter

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestGroupMessages_SplitsToolSequence(t *testing.T) {
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("u1").Name("Alice").Build(),
		message.NewMsg().Role(message.RoleAssistant).Content(message.NewToolUseBlock("t1", "echo", map[string]any{})).Build(),
		message.NewMsg().Role(message.RoleTool).Content(message.NewToolResultBlock("t1", []message.ContentBlock{message.NewTextBlock("ok")}, false)).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("u2").Name("Bob").Build(),
	}
	groups := GroupMessages(msgs)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if groups[0].Type != "agent_message" || groups[1].Type != "tool_sequence" || groups[2].Type != "agent_message" {
		t.Fatalf("unexpected group order/types: %+v", groups)
	}
}

func TestFormatOpenAIMultiAgentMessages_WrapsHistory(t *testing.T) {
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("sys").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Name("Alice").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("hi").Name("Bot").Build(),
	}
	out := FormatOpenAIMultiAgentMessages(NewOpenAIFormatter(), msgs)
	if len(out) != 2 {
		t.Fatalf("expected system + wrapped history, got %d messages", len(out))
	}
	if out[0].Role != "system" {
		t.Fatalf("expected system first, got %s", out[0].Role)
	}
	if out[1].Role != "user" || !containsAll(out[1].Content, "<history>", "Alice: hello", "Bot: hi", "</history>") {
		t.Fatalf("unexpected wrapped history: %q", out[1].Content)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
