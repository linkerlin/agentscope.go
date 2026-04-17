package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileTool_CreateNew(t *testing.T) {
	tool := NewWriteFileTool("")
	path := filepath.Join(t.TempDir(), "new.txt")
	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": path, "content": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Create and write") {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %s", string(data))
	}
}

func TestWriteFileTool_Overwrite(t *testing.T) {
	tool := NewWriteFileTool("")
	f, _ := os.CreateTemp("", "write*.txt")
	f.WriteString("old")
	f.Close()
	defer os.Remove(f.Name())

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": f.Name(), "content": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Overwrite") {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
	data, _ := os.ReadFile(f.Name())
	if string(data) != "new" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestWriteFileTool_ReplaceRange(t *testing.T) {
	tool := NewWriteFileTool("")
	f, _ := os.CreateTemp("", "write*.txt")
	f.WriteString("line1\nline2\nline3\nline4\n")
	f.Close()
	defer os.Remove(f.Name())

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": f.Name(), "content": "replaced", "ranges": "2,3"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f.Name())
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[1] != "replaced" {
		t.Fatalf("unexpected content: %s", string(data))
	}
	if !strings.Contains(resp.GetTextContent(), "Write") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestInsertTextFileTool(t *testing.T) {
	tool := NewInsertTextFileTool("")
	f, _ := os.CreateTemp("", "insert*.txt")
	f.WriteString("a\nb\nc\n")
	f.Close()
	defer os.Remove(f.Name())

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": f.Name(), "content": "x", "line_number": float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f.Name())
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 || lines[1] != "x" {
		t.Fatalf("unexpected content: %s", string(data))
	}
	if !strings.Contains(resp.GetTextContent(), "Insert content") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
}
