package workspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewBubblewrapWorkspace(t *testing.T) {
	dir := t.TempDir()
	w, err := NewBubblewrapWorkspace(BubblewrapConfig{
		ID:      "bw1",
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("NewBubblewrapWorkspace: %v", err)
	}
	if w.ID() != "bw1" {
		t.Fatalf("expected ID 'bw1', got %s", w.ID())
	}
	if w.Type() != "bubblewrap" {
		t.Fatalf("expected Type 'bubblewrap', got %s", w.Type())
	}
}

func TestNewBubblewrapWorkspace_NoBaseDir(t *testing.T) {
	_, err := NewBubblewrapWorkspace(BubblewrapConfig{ID: "bw1"})
	if err == nil {
		t.Fatal("should error without BaseDir")
	}
}

func TestBubblewrapWorkspace_FileOps(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{ID: "bw1", BaseDir: dir})
	ctx := context.Background()

	// WriteFile
	if err := w.WriteFile(ctx, "test.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// ReadFile
	data, err := w.ReadFile(ctx, "test.txt")
	if err != nil || string(data) != "hello" {
		t.Fatalf("ReadFile: %v data=%s", err, string(data))
	}

	// Stat
	info, err := w.Stat(ctx, "test.txt")
	if err != nil || info.Name != "test.txt" {
		t.Fatalf("Stat: %v %+v", err, info)
	}

	// MkdirAll + ListDir
	w.MkdirAll(ctx, "subdir", 0o755)
	w.WriteFile(ctx, "subdir/nested.txt", []byte("nested"), 0o644)
	entries, err := w.ListDir(ctx, "subdir")
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListDir: %v len=%d", err, len(entries))
	}
	if entries[0].Name != "nested.txt" {
		t.Fatalf("expected nested.txt, got %s", entries[0].Name)
	}
}

func TestBubblewrapWorkspace_BuildBwrapArgs(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{
		ID:      "bw1",
		BaseDir: dir,
	})

	args := w.buildBwrapArgs("")

	// Must contain bind mount for workspace
	foundBind := false
	for i, a := range args {
		if a == "--bind" && i+2 < len(args) && args[i+2] == "/workspace" {
			foundBind = true
			break
		}
	}
	if !foundBind {
		t.Fatalf("args should contain --bind <host> /workspace: %v", args)
	}

	// Must contain --die-with-parent
	foundDie := false
	for _, a := range args {
		if a == "--die-with-parent" {
			foundDie = true
			break
		}
	}
	if !foundDie {
		t.Fatalf("args should contain --die-with-parent: %v", args)
	}

	// Must contain --chdir /workspace
	foundChdir := false
	for i, a := range args {
		if a == "--chdir" && i+1 < len(args) && args[i+1] == "/workspace" {
			foundChdir = true
			break
		}
	}
	if !foundChdir {
		t.Fatalf("args should contain --chdir /workspace: %v", args)
	}
}

func TestBubblewrapWorkspace_BuildBwrapArgs_WithWorkingDir(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{
		ID:      "bw1",
		BaseDir: dir,
	})

	args := w.buildBwrapArgs("projects/myapp")

	foundChdir := false
	for i, a := range args {
		if a == "--chdir" && i+1 < len(args) {
			expected := "/workspace/projects/myapp"
			if args[i+1] == expected {
				foundChdir = true
			}
			break
		}
	}
	if !foundChdir {
		t.Fatalf("args should contain --chdir /workspace/projects/myapp: %v", args)
	}
}

func TestBubblewrapWorkspace_BuildBwrapArgs_ShareNet(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{
		ID:       "bw1",
		BaseDir:  dir,
		ShareNet: true,
	})

	args := w.buildBwrapArgs("")
	for _, a := range args {
		if a == "--unshare-net" {
			t.Fatal("ShareNet=true should not add --unshare-net")
		}
	}
}

func TestBubblewrapWorkspace_BuildBwrapArgs_UnshareNet(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{
		ID:      "bw1",
		BaseDir: dir,
	})

	args := w.buildBwrapArgs("")
	found := false
	for _, a := range args {
		if a == "--unshare-net" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ShareNet=false (default) should add --unshare-net")
	}
}

func TestBubblewrapWorkspace_BuildBwrapArgs_CustomReadOnly(t *testing.T) {
	dir := t.TempDir()
	// Create a custom read-only dir
	customDir := filepath.Join(dir, "custom")
	os.MkdirAll(customDir, 0o755)

	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{
		ID:      "bw1",
		BaseDir: dir,
		ReadOnlyDirs: map[string]string{
			customDir: "/custom",
		},
	})

	args := w.buildBwrapArgs("")
	foundCustom := false
	for i, a := range args {
		if a == "--ro-bind" && i+2 < len(args) && args[i+2] == "/custom" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatalf("args should contain custom ro-bind: %v", args)
	}
}

func TestBubblewrapWorkspace_Close(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{ID: "bw1", BaseDir: dir})
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBubblewrapWorkspace_Execute_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("test for non-Linux only")
	}
	dir := t.TempDir()
	w, _ := NewBubblewrapWorkspace(BubblewrapConfig{ID: "bw1", BaseDir: dir})
	_, err := w.Execute(context.Background(), "echo hello", ExecuteOptions{})
	if err == nil {
		t.Fatal("should error on non-Linux")
	}
	if !strings.Contains(err.Error(), "Linux") {
		t.Fatalf("error should mention Linux: %v", err)
	}
}
