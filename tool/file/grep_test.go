package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepTool_FilesWithMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.py"), []byte("def main():\n    pass\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func",
		"path":        dir,
		"output_mode": "files_with_matches",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "a.go") {
		t.Fatalf("expected a.go, got: %s", text)
	}
	if strings.Contains(text, "b.py") {
		t.Fatalf("unexpected b.py in output: %s", text)
	}
}

func TestGrepTool_Content(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\nfunc hello() {}\nfunc world() {}\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func",
		"path":        dir,
		"output_mode": "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "2: func hello() {}") {
		t.Fatalf("expected line 2 match, got: %s", text)
	}
	if !strings.Contains(text, "3: func world() {}") {
		t.Fatalf("expected line 3 match, got: %s", text)
	}
}

func TestGrepTool_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("Hello World\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "files_with_matches",
		"i":           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "test.txt") {
		t.Fatalf("expected match, got: %s", resp.GetTextContent())
	}
}

func TestGrepTool_GlobFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("func main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.py"), []byte("func main() {}\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func",
		"path":        dir,
		"output_mode": "files_with_matches",
		"glob":        "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "a.go") {
		t.Fatalf("expected a.go, got: %s", text)
	}
	if strings.Contains(text, "b.py") {
		t.Fatalf("unexpected b.py, got: %s", text)
	}
}

func TestGrepTool_Count(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("func a() {}\nfunc b() {}\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func",
		"path":        dir,
		"output_mode": "count",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), ": 2") {
		t.Fatalf("expected count 2, got: %s", resp.GetTextContent())
	}
}

func TestGrepTool_Context(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("line1\nline2\nfunc main() {}\nline4\nline5\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func",
		"path":        dir,
		"output_mode": "content",
		"context":     float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "line2") || !strings.Contains(text, "func main()") || !strings.Contains(text, "line4") {
		t.Fatalf("expected context lines, got: %s", text)
	}
}

func TestGrepTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\n"), 0o644)

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "nonexistent_pattern_xyz",
		"path":    dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "No matches found") {
		t.Fatalf("expected no matches message, got: %s", resp.GetTextContent())
	}
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	tool := NewGrepTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "[invalid",
		"path":    ".",
	})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestGrepTool_BaseDirRestriction(t *testing.T) {
	dir := t.TempDir()
	tool := NewGrepTool(dir)
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "test",
		"path":    "..",
	})
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestGrepTool_HeadLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.go", i)), []byte("func main() {}\n"), 0o644)
	}

	tool := NewGrepTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func",
		"path":        dir,
		"output_mode": "files_with_matches",
		"head_limit":  float64(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(resp.GetTextContent()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 results, got %d: %s", len(lines), resp.GetTextContent())
	}
}
