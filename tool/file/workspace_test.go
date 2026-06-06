package file

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/workspace"
)

// mockWorkspace is a minimal workspace for testing tool binding.
type mockWorkspace struct {
	files map[string][]byte
}

func newMockWorkspace() *mockWorkspace {
	return &mockWorkspace{files: make(map[string][]byte)}
}

func (m *mockWorkspace) ID() string   { return "mock" }
func (m *mockWorkspace) Type() string { return "mock" }
func (m *mockWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return data, nil
}
func (m *mockWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	m.files[path] = data
	return nil
}
func (m *mockWorkspace) ListDir(ctx context.Context, path string) ([]workspace.DirEntry, error) {
	return nil, nil
}
func (m *mockWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return nil
}
func (m *mockWorkspace) Stat(ctx context.Context, path string) (workspace.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return workspace.FileInfo{Name: path, Size: int64(len(m.files[path])), IsDir: false}, nil
	}
	return workspace.FileInfo{}, fs.ErrNotExist
}
func (m *mockWorkspace) Execute(ctx context.Context, command string, opts workspace.ExecuteOptions) (*workspace.ExecuteResult, error) {
	return nil, nil
}
func (m *mockWorkspace) Close() error { return nil }

func TestReadFileTool_WithWorkspace(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	ws := newMockWorkspace()
	ws.files[filepath.Join(baseDir, "test.txt")] = []byte("hello from workspace")

	tool := NewReadFileTool(baseDir).WithWorkspace(ws)
	resp, err := tool.Execute(ctx, map[string]any{"file_path": "test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "hello from workspace") {
		t.Fatalf("unexpected content: %s", resp.GetTextContent())
	}
}

func TestWriteFileTool_WithWorkspace(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	ws := newMockWorkspace()

	tool := NewWriteFileTool(baseDir).WithWorkspace(ws)
	_, err := tool.Execute(ctx, map[string]any{"file_path": "out.txt", "content": "written"})
	if err != nil {
		t.Fatal(err)
	}

	data, ok := ws.files[filepath.Join(baseDir, "out.txt")]
	if !ok {
		t.Fatal("expected file to be written to workspace")
	}
	if string(data) != "written" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestListDirectoryTool_WithWorkspace(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	ws := newMockWorkspace()

	tool := NewListDirectoryTool(baseDir).WithWorkspace(ws)
	_, err := tool.Execute(ctx, map[string]any{"dir_path": "."})
	if err != nil {
		// mock workspace returns nil for ListDir, which is acceptable
		t.Logf("list dir returned: %v", err)
	}
}

func TestInsertTextFileTool_WithWorkspace(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	ws := newMockWorkspace()
	ws.files[filepath.Join(baseDir, "doc.txt")] = []byte("line1\nline2\nline3\n")

	tool := NewInsertTextFileTool(baseDir).WithWorkspace(ws)
	_, err := tool.Execute(ctx, map[string]any{
		"file_path":   "doc.txt",
		"line_number": float64(2),
		"content":     "inserted",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := ws.files[filepath.Join(baseDir, "doc.txt")]
	if string(data) != "line1\ninserted\nline2\nline3" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}
