package permission

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	"github.com/linkerlin/agentscope.go/tool/shell"
)

func TestEngine_ToolResolver_FileMatchRule(t *testing.T) {
	readTool := file.NewReadFileTool("")
	e := NewEngine(ModeDefault, []Rule{
		{Name: "allow-src", ToolName: "view_text_file", Pattern: "src/**", Decision: DecisionAllow},
	})
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "view_text_file" {
			return readTool
		}
		return nil
	})

	evals, err := e.Evaluate([]*message.ToolUseBlock{{
		Name:  "view_text_file",
		Input: map[string]any{"file_path": "src/main.go"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow via file match rule, got %s", evals[0].Decision)
	}
}

func TestEngine_ToolResolver_BashPrefixRule(t *testing.T) {
	sh := shell.NewShellCommandTool("", nil, nil)
	e := NewEngine(ModeDefault, []Rule{
		{Name: "allow-git", ToolName: "execute_shell_command", Pattern: "git:*", Decision: DecisionAllow},
	})
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "execute_shell_command" {
			return sh
		}
		return nil
	})

	evals, err := e.Evaluate([]*message.ToolUseBlock{{
		Name:  "execute_shell_command",
		Input: map[string]any{"command": "git status"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow via bash prefix rule, got %s", evals[0].Decision)
	}
}

func TestEngine_ToolResolver_FileSuggestions(t *testing.T) {
	editTool := file.NewEditFileTool("")
	e := NewEngine(ModeDefault, nil)
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "edit_text_file" {
			return editTool
		}
		return nil
	})

	evals, _ := e.Evaluate([]*message.ToolUseBlock{{
		Name:  "edit_text_file",
		Input: map[string]any{"file_path": "pkg/a.go"},
	}})
	if len(evals[0].SuggestedRules) == 0 {
		t.Fatal("expected suggestions")
	}
	if evals[0].SuggestedRules[0].Pattern != "pkg/**" {
		t.Fatalf("unexpected suggestion: %+v", evals[0].SuggestedRules[0])
	}
}
