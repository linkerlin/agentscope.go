package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestShellCommandTool_WhitelistAllow(t *testing.T) {
	s := NewShellCommandTool("", []string{"echo"}, nil)
	resp, err := s.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "hello") {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
}

func TestShellCommandTool_WhitelistReject(t *testing.T) {
	s := NewShellCommandTool("", []string{"echo"}, nil)
	resp, err := s.Execute(context.Background(), map[string]any{"command": "whoami"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "SecurityError") {
		t.Fatalf("expected security error, got: %s", resp.GetTextContent())
	}
}

func TestShellCommandTool_ApprovalCallback(t *testing.T) {
	approved := false
	s := NewShellCommandTool("", []string{"echo"}, func(cmd string) bool {
		approved = true
		return true
	})
	resp, err := s.Execute(context.Background(), map[string]any{"command": "whoami"})
	if err != nil {
		t.Fatal(err)
	}
	if !approved {
		t.Fatal("expected approval callback to be called")
	}
	// on windows whoami works, on unix too
	if strings.Contains(resp.GetTextContent(), "SecurityError") {
		t.Fatalf("expected approval to allow execution, got: %s", resp.GetTextContent())
	}
}

func TestShellCommandTool_MultipleCommandsBlocked(t *testing.T) {
	s := NewShellCommandTool("", []string{"echo"}, nil)
	resp, err := s.Execute(context.Background(), map[string]any{"command": "echo a && echo b"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "SecurityError") {
		t.Fatalf("expected security error for multiple commands, got: %s", resp.GetTextContent())
	}
}

func TestShellCommandTool_EmptyCommand(t *testing.T) {
	s := NewShellCommandTool("", nil, nil)
	_, err := s.Execute(context.Background(), map[string]any{"command": "  "})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestShellCommandTool_Timeout(t *testing.T) {
	s := NewShellCommandTool("", nil, nil)
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "timeout /t 3 /nobreak >nul"
	} else {
		cmd = "sleep 3"
	}
	resp, err := s.Execute(context.Background(), map[string]any{"command": cmd, "timeout": 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "TimeoutError") {
		t.Fatalf("expected timeout error, got: %s", resp.GetTextContent())
	}
}

func TestShellCommandTool_WorkingDir(t *testing.T) {
	s := NewShellCommandTool("", nil, nil)
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cd"
	} else {
		cmd = "pwd"
	}
	wd := t.TempDir()
	resp, err := s.Execute(context.Background(), map[string]any{"command": cmd, "working_dir": wd})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), wd) {
		t.Fatalf("expected working dir in output, got: %s", resp.GetTextContent())
	}
}
