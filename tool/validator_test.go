package tool

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestValidateToolResultMatch_NoPendingTools(t *testing.T) {
	assistant := message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build()
	if err := ValidateToolResultMatch(assistant, nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateToolResultMatch_MissingResults(t *testing.T) {
	assistant := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "search", map[string]any{"q": "go"}),
	).Build()
	err := ValidateToolResultMatch(assistant, nil)
	if err == nil {
		t.Fatal("expected error for missing tool results")
	}
}

func TestValidateToolResultMatch_MatchingResults(t *testing.T) {
	assistant := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "search", map[string]any{"q": "go"}),
	).Build()
	input := []*message.Msg{
		message.NewMsg().Role(message.RoleTool).Content(
			message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock("ok")}, false),
		).Build(),
	}
	if err := ValidateToolResultMatch(assistant, input); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateToolResultMatch_PartialResults(t *testing.T) {
	assistant := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "search", map[string]any{"q": "go"}),
		message.NewToolUseBlock("call_2", "calc", map[string]any{"expr": "1+1"}),
	).Build()
	input := []*message.Msg{
		message.NewMsg().Role(message.RoleTool).Content(
			message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock("ok")}, false),
		).Build(),
	}
	err := ValidateToolResultMatch(assistant, input)
	if err == nil {
		t.Fatal("expected error for partial tool results")
	}
}
