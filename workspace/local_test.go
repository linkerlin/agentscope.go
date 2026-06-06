package workspace

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalWorkspace_ReadWriteFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	// Write
	path := filepath.Join("subdir", "test.txt")
	data := []byte("hello world")
	if err := ws.WriteFile(ctx, path, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Read
	got, err := ws.ReadFile(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Fatalf("content mismatch: %s", string(got))
	}

	// Stat
	info, err := ws.Stat(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "test.txt" {
		t.Fatalf("name mismatch: %s", info.Name)
	}
	if info.Size != int64(len(data)) {
		t.Fatalf("size mismatch: %d", info.Size)
	}
	if info.IsDir {
		t.Fatal("expected file, got dir")
	}
}

func TestLocalWorkspace_ListDir(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	_ = ws.WriteFile(ctx, "a.txt", []byte("a"), 0644)
	_ = ws.WriteFile(ctx, "b.txt", []byte("b"), 0644)
	_ = ws.MkdirAll(ctx, "subdir", 0755)

	entries, err := ws.ListDir(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	foundDir := false
	for _, e := range entries {
		if e.Name == "subdir" && e.IsDir {
			foundDir = true
		}
	}
	if !foundDir {
		t.Fatal("expected subdir in entries")
	}
}

func TestLocalWorkspace_Execute(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	// Write a script
	script := "#!/bin/sh\necho hello from workspace\n"
	if err := ws.WriteFile(ctx, "test.sh", []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Execute (cross-platform: use echo)
	res, err := ws.Execute(ctx, "echo test-output", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", res.ExitCode)
	}
	if res.Stdout != "test-output\n" && res.Stdout != "test-output\r\n" {
		// Windows may produce CRLF
		if !contains(res.Stdout, "test-output") {
			t.Fatalf("unexpected stdout: %q", res.Stdout)
		}
	}
}

func TestLocalWorkspace_PathTraversalBlocked(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	// Attempt to read outside workspace
	_, err := ws.ReadFile(ctx, "../outside.txt")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestLocalWorkspace_ReadFileNotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	_, err := ws.ReadFile(ctx, "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLocalWorkspace_MkdirAll(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	if err := ws.MkdirAll(ctx, "a/b/c", 0755); err != nil {
		t.Fatal(err)
	}

	info, err := ws.Stat(ctx, "a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir {
		t.Fatal("expected directory")
	}
}

func TestLocalWorkspace_ExecuteTimeout(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := NewLocalWorkspace("test", dir)
	defer ws.Close()

	// Use a short sleep so the test doesn't hang even if timeout propagation
	// is imperfect on the host OS (e.g. Windows child-process behaviour).
	res, err := ws.Execute(ctx, "sleep 1", ExecuteOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	// Timeout should kill the process; exit code may be non-zero.
	// We mainly care that it returns without hanging for the full sleep.
	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit code for timed-out command")
	}
}

func TestLocalWorkspace_IDAndType(t *testing.T) {
	ws := NewLocalWorkspace("test", "/tmp/test")
	if ws.ID() == "" {
		t.Fatal("expected non-empty ID")
	}
	if ws.Type() != "local" {
		t.Fatalf("expected type local, got %s", ws.Type())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
