package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTool_ViewFull(t *testing.T) {
	tool := NewReadFileTool("")
	f, _ := os.CreateTemp("", "read*.txt")
	f.WriteString("hello\nworld\n")
	f.Close()
	defer os.Remove(f.Name())

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "hello") {
		t.Fatal("expected hello in output")
	}
}

func TestReadFileTool_ViewRange(t *testing.T) {
	tool := NewReadFileTool("")
	f, _ := os.CreateTemp("", "read*.txt")
	for i := 1; i <= 10; i++ {
		f.WriteString("line\n")
	}
	f.Close()
	defer os.Remove(f.Name())

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": f.Name(), "ranges": "3,5"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "3: line") {
		t.Fatal("expected line 3")
	}
}

func TestReadFileTool_ViewNegativeRange(t *testing.T) {
	tool := NewReadFileTool("")
	f, _ := os.CreateTemp("", "read*.txt")
	f.WriteString("a\nb\nc\nd\n")
	f.Close()
	defer os.Remove(f.Name())

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": f.Name(), "ranges": "-2,-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "3: c") || !strings.Contains(resp.GetTextContent(), "4: d") {
		t.Fatalf("unexpected output: %s", resp.GetTextContent())
	}
}

func TestReadFileTool_BaseDirRestriction(t *testing.T) {
	dir := t.TempDir()
	tool := NewReadFileTool(dir)
	f, _ := os.CreateTemp(dir, "safe*.txt")
	f.WriteString("ok")
	f.Close()

	resp, err := tool.Execute(context.Background(), map[string]any{"file_path": filepath.Base(f.Name())})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "ok") {
		t.Fatal("expected content")
	}

	_, err = tool.Execute(context.Background(), map[string]any{"file_path": "../outside.txt"})
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestListDirectoryTool(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)

	tool := NewListDirectoryTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{"dir_path": dir})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "a.txt") || !strings.Contains(text, "sub") {
		t.Fatalf("unexpected output: %s", text)
	}
}
