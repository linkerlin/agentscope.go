package react

import (
	"context"
	"io/fs"
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	"github.com/linkerlin/agentscope.go/tool/shell"
	"github.com/linkerlin/agentscope.go/workspace"
)

// trackingWorkspace records whether it was ever used.
type trackingWorkspace struct {
	used bool
}

func (t *trackingWorkspace) ID() string   { return "track" }
func (t *trackingWorkspace) Type() string { return "tracking" }
func (t *trackingWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	t.used = true
	return []byte("mock content"), nil
}
func (t *trackingWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	t.used = true
	return nil
}
func (t *trackingWorkspace) ListDir(ctx context.Context, path string) ([]workspace.DirEntry, error) {
	t.used = true
	return []workspace.DirEntry{{Name: "a.txt", IsDir: false}}, nil
}
func (t *trackingWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	t.used = true
	return nil
}
func (t *trackingWorkspace) Stat(ctx context.Context, path string) (workspace.FileInfo, error) {
	t.used = true
	return workspace.FileInfo{Name: path}, nil
}
func (t *trackingWorkspace) Execute(ctx context.Context, command string, opts workspace.ExecuteOptions) (*workspace.ExecuteResult, error) {
	t.used = true
	return &workspace.ExecuteResult{ExitCode: 0, Stdout: "ok"}, nil
}
func (t *trackingWorkspace) Close() error { return nil }

func TestBindWorkspaceToTool_AllTools(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		tool    tool.Tool
		execute func(tool.Tool) error
	}{
		{
			name: "ReadFileTool",
			tool: file.NewReadFileTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{"file_path": "/test.txt"})
				return err
			},
		},
		{
			name: "WriteFileTool",
			tool: file.NewWriteFileTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{"file_path": "/test.txt", "content": "hello"})
				return err
			},
		},
		{
			name: "InsertTextFileTool",
			tool: file.NewInsertTextFileTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{
					"file_path":   "/test.txt",
					"line_number": float64(1),
					"content":     "inserted",
				})
				return err
			},
		},
		{
			name: "ListDirectoryTool",
			tool: file.NewListDirectoryTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{"dir_path": "/tmp"})
				return err
			},
		},
		{
			name: "EditFileTool",
			tool: file.NewEditFileTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{
					"file_path":   "/test.txt",
					"old_string":  "old",
					"new_string":  "new",
					"replace_all": false,
				})
				return err
			},
		},
		{
			name: "GlobTool",
			tool: file.NewGlobTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{"pattern": "*.txt"})
				return err
			},
		},
		{
			name: "GrepTool",
			tool: file.NewGrepTool("/tmp"),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{"pattern": "hello", "path": "/tmp"})
				return err
			},
		},
		{
			name: "ShellCommandTool",
			tool: shell.NewShellCommandTool("/tmp", nil, nil),
			execute: func(tl tool.Tool) error {
				_, err := tl.Execute(ctx, map[string]any{"command": "echo ok"})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := &trackingWorkspace{}
			bindWorkspaceToTool(tc.tool, ws)

			_ = tc.execute(tc.tool)

			if !ws.used {
				t.Fatalf("tool %s did not use the bound workspace", tc.name)
			}
		})
	}
}
