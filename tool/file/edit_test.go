package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileTool_SingleReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("hello world\nfoo bar\n"), 0o644)

	tool := NewEditFileTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "foo",
		"new_string": "baz",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Successfully replaced") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello world\nbaz bar\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestEditFileTool_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("foo foo foo\n"), 0o644)

	tool := NewEditFileTool("")
	resp, err := tool.Execute(context.Background(), map[string]any{
		"file_path":   path,
		"old_string":  "foo",
		"new_string":  "bar",
		"replace_all": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "all 3 occurrences") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "bar bar bar\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestEditFileTool_MultipleWithoutReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("foo foo\n"), 0o644)

	tool := NewEditFileTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "foo",
		"new_string": "bar",
	})
	if err == nil {
		t.Fatal("expected error for multiple occurrences without replace_all")
	}
	if !strings.Contains(err.Error(), "appears 2 times") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditFileTool_OldStringNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("hello world\n"), 0o644)

	tool := NewEditFileTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "missing",
		"new_string": "replacement",
	})
	if err == nil {
		t.Fatal("expected error for missing old_string")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditFileTool_IdenticalStrings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("hello\n"), 0o644)

	tool := NewEditFileTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "same",
		"new_string": "same",
	})
	if err == nil {
		t.Fatal("expected error for identical strings")
	}
}

func TestEditFileTool_BaseDirRestriction(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "safe.txt"), []byte("inside\n"), 0o644)

	tool := NewEditFileTool(dir)
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  "../outside.txt",
		"old_string": "a",
		"new_string": "b",
	})
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestEditFileTool_EmptyOldString(t *testing.T) {
	tool := NewEditFileTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  "/tmp/test.txt",
		"old_string": "",
		"new_string": "x",
	})
	if err == nil {
		t.Fatal("expected error for empty old_string")
	}
}
