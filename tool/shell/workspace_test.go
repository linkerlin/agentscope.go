package shell

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/workspace"
)

// mockWorkspace is a minimal workspace for testing shell tool binding.
type mockWorkspace struct {
	lastCommand string
	lastOpts    workspace.ExecuteOptions
}

func newMockWorkspace() *mockWorkspace {
	return &mockWorkspace{}
}

func (m *mockWorkspace) ID() string   { return "mock" }
func (m *mockWorkspace) Type() string { return "mock" }
func (m *mockWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, fs.ErrNotExist
}
func (m *mockWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	return nil
}
func (m *mockWorkspace) ListDir(ctx context.Context, path string) ([]workspace.DirEntry, error) {
	return nil, nil
}
func (m *mockWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return nil
}
func (m *mockWorkspace) Stat(ctx context.Context, path string) (workspace.FileInfo, error) {
	return workspace.FileInfo{}, fs.ErrNotExist
}
func (m *mockWorkspace) Execute(ctx context.Context, command string, opts workspace.ExecuteOptions) (*workspace.ExecuteResult, error) {
	m.lastCommand = command
	m.lastOpts = opts
	return &workspace.ExecuteResult{
		ExitCode: 0,
		Stdout:   "mock-stdout",
		Stderr:   "",
	}, nil
}
func (m *mockWorkspace) Close() error { return nil }

func TestShellCommandTool_WithWorkspace(t *testing.T) {
	ctx := context.Background()
	ws := newMockWorkspace()

	s := NewShellCommandTool("", []string{"echo"}, nil).WithWorkspace(ws)
	resp, err := s.Execute(ctx, map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatal(err)
	}

	if ws.lastCommand != "echo hello" {
		t.Fatalf("expected command delegated to workspace, got: %s", ws.lastCommand)
	}
	if !strings.Contains(resp.GetTextContent(), "mock-stdout") {
		t.Fatalf("expected workspace stdout, got: %s", resp.GetTextContent())
	}
}

func TestShellCommandTool_WithWorkspace_WorkingDir(t *testing.T) {
	ctx := context.Background()
	ws := newMockWorkspace()

	s := NewShellCommandTool("/base", []string{"echo"}, nil).WithWorkspace(ws)
	_, err := s.Execute(ctx, map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatal(err)
	}

	if ws.lastOpts.WorkingDir != "/base" {
		t.Fatalf("expected working dir /base, got: %s", ws.lastOpts.WorkingDir)
	}
}

func TestShellCommandTool_WithWorkspace_CustomWorkingDir(t *testing.T) {
	ctx := context.Background()
	ws := newMockWorkspace()

	s := NewShellCommandTool("/base", []string{"echo"}, nil).WithWorkspace(ws)
	_, err := s.Execute(ctx, map[string]any{"command": "echo hello", "working_dir": "/custom"})
	if err != nil {
		t.Fatal(err)
	}

	if ws.lastOpts.WorkingDir != "/custom" {
		t.Fatalf("expected working dir /custom, got: %s", ws.lastOpts.WorkingDir)
	}
}
