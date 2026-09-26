package permission

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/askuser"
)

// TestEngine_AutoApprovedTool verifies tools that declare themselves
// auto-approved (AskUser) never prompt, even in default mode.
func TestEngine_AutoApprovedTool(t *testing.T) {
	e := NewEngine(ModeDefault, nil)
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "ask_user" {
			return askuser.New()
		}
		return nil
	})
	evals, err := e.Evaluate([]*message.ToolUseBlock{
		{Name: "ask_user", Input: map[string]any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected auto-approved allow, got %s", evals[0].Decision)
	}
}

// TestEngine_AutoApprovedTool_DenyWins verifies explicit deny rules still take
// priority over auto approval.
func TestEngine_AutoApprovedTool_DenyWins(t *testing.T) {
	e := NewEngine(ModeDefault, []Rule{
		{Name: "no-ask", ToolName: "ask_user", Decision: DecisionDeny},
	})
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "ask_user" {
			return askuser.New()
		}
		return nil
	})
	evals, err := e.Evaluate([]*message.ToolUseBlock{
		{Name: "ask_user", Input: map[string]any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionDeny {
		t.Fatalf("expected deny rule to win, got %s", evals[0].Decision)
	}
}
