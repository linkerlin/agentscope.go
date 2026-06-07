package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGlobTool_BasicPattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0o644)

	tool := NewGlobTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "a.txt") || !strings.Contains(text, "b.txt") {
		t.Fatalf("expected a.txt and b.txt, got: %s", text)
	}
	if strings.Contains(text, "c.go") {
		t.Fatalf("unexpected c.go in output: %s", text)
	}
}

func TestGlobTool_RecursivePattern(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "root.go"), []byte("r"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "nested.go"), []byte("n"), 0o644)

	tool := NewGlobTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "**/*.go",
		"path":    dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "root.go") || !strings.Contains(text, "nested.go") {
		t.Fatalf("expected root.go and nested.go, got: %s", text)
	}
}

func TestGlobTool_SortByModTime(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "older.txt"), []byte("o"), 0o644)
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "newer.txt"), []byte("n"), 0o644)

	tool := NewGlobTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(resp.GetTextContent(), "\n")
	if len(lines) < 2 {
		t.Fatal("expected at least 2 matches")
	}
	if !strings.Contains(lines[0], "newer.txt") {
		t.Fatalf("expected newer.txt first, got: %s", lines[0])
	}
}

func TestGlobTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)

	tool := NewGlobTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.py",
		"path":    dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "No files found") {
		t.Fatalf("expected no files message, got: %s", resp.GetTextContent())
	}
}

func TestGlobTool_BaseDirRestriction(t *testing.T) {
	dir := t.TempDir()
	tool := NewGlobTool(dir)
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    "..",
	})
	if err == nil {
		t.Fatal("expected traversal error")
	}
}
