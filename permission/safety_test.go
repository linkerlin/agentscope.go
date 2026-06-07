package permission

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestIsDangerousPath(t *testing.T) {
	files := []string{".bashrc", ".env"}
	dirs := []string{".git", ".ssh"}

	cases := []struct {
		path string
		want bool
	}{
		{"/home/user/.bashrc", true},
		{"/home/user/.env", true},
		{"/home/user/.env.local", false}, // not in the short list
		{"/home/user/project/.git/config", true},
		{"/home/user/.ssh/authorized_keys", true},
		{"/home/user/project/main.py", false},
		{"", false},
	}

	for _, c := range cases {
		got := IsDangerousPath(c.path, files, dirs)
		if got != c.want {
			t.Fatalf("IsDangerousPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsDangerousPath_Defaults(t *testing.T) {
	if !IsDangerousPath("/home/user/.bashrc", DefaultDangerousFiles, DefaultDangerousDirs) {
		t.Fatal(".bashrc should be dangerous with defaults")
	}
	if !IsDangerousPath("/repo/.git/config", DefaultDangerousFiles, DefaultDangerousDirs) {
		t.Fatal(".git directory should be dangerous with defaults")
	}
}

func TestIsDangerousCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"rm -rf /", true},
		{"sudo rm /etc/passwd", true},
		{"chmod 777 /tmp", true},
		{"chmod -R 777 /tmp", true},
		{"kill -9 1234", true},
		{"ls -la", false},
		{"git status", false},
		{"", false},
	}

	for _, c := range cases {
		got := IsDangerousCommand(c.cmd)
		if got != c.want {
			t.Fatalf("IsDangerousCommand(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestIsReadOnlyCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"ls -la", true},
		{"cat file.txt", true},
		{"git status", true},
		{"git log --oneline", true},
		{"pwd", true},
		{"echo hello", true},
		{"rm -rf /", false},
		{"git commit -m 'x'", false},
		{"lsd", false}, // prefix match should not match longer commands
	}

	for _, c := range cases {
		got := IsReadOnlyCommand(c.cmd)
		if got != c.want {
			t.Fatalf("IsReadOnlyCommand(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestIsDangerousRemoval(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"rm -rf /", true},
		{"rm -rf /usr", true},
		{"rm -rf ~", true},
		{"rm -rf *", true},
		{"rm file.txt", false},
		{"rm /home/user/file.txt", false},
		{"rmdir /tmp/dir", false},
		{"ls -la", false},
		{"rmdir ~", true},
	}

	for _, c := range cases {
		got := IsDangerousRemoval(c.cmd)
		if got != c.want {
			t.Fatalf("IsDangerousRemoval(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestEngine_SafetyChecks_DangerousCommand(t *testing.T) {
	e := NewEngine(ModeBypass, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{
		{Name: "execute_shell_command", Input: map[string]any{"command": "rm -rf /"}},
	})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask for dangerous command, got %s", evals[0].Decision)
	}
}

func TestEngine_SafetyChecks_ReadOnlyCommand(t *testing.T) {
	e := NewEngine(ModeDefault, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{
		{Name: "execute_shell_command", Input: map[string]any{"command": "ls -la"}},
	})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow for read-only command, got %s", evals[0].Decision)
	}
}

func TestEngine_SafetyChecks_DangerousPath(t *testing.T) {
	e := NewEngine(ModeBypass, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{
		{Name: "write_text_file", Input: map[string]any{"file_path": "/home/user/.bashrc"}},
	})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask for dangerous path, got %s", evals[0].Decision)
	}
}

func TestEngine_DEFAULT_Mode(t *testing.T) {
	e := NewEngine(ModeDefault, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "write_text_file"}})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask in default mode, got %s", evals[0].Decision)
	}
}

func TestEngine_EXPLORE_DenyModification(t *testing.T) {
	e := NewEngine(ModeExplore, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "write_text_file"}})
	if evals[0].Decision != DecisionDeny {
		t.Fatalf("expected deny in explore mode, got %s", evals[0].Decision)
	}
}

func TestEngine_ACCEPT_EDITS_WorkingDir(t *testing.T) {
	ctx := NewContext(ModeAcceptEdits).WithWorkingDirs("/tmp/workspace")
	e := NewEngineWithContext(ctx, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{
		{Name: "write_text_file", Input: map[string]any{"file_path": "/tmp/workspace/main.go"}},
	})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow for file in working dir, got %s", evals[0].Decision)
	}
}

func TestEngine_SuggestedRules(t *testing.T) {
	e := NewEngine(ModeDefault, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{
		{Name: "execute_shell_command", Input: map[string]any{"command": "git commit -m 'hello'"}},
	})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask in default mode, got %s", evals[0].Decision)
	}
	if len(evals[0].SuggestedRules) == 0 {
		t.Fatal("expected suggested rules")
	}
	if evals[0].SuggestedRules[0].Pattern != "git commit:*" {
		t.Fatalf("expected suggestion 'git commit:*', got %s", evals[0].SuggestedRules[0].Pattern)
	}
}
