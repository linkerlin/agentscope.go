package workspace

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestK8sWorkspace_FileOps(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("file content", "", 0),
	}

	ctx := context.Background()

	// ReadFile
	data, err := w.ReadFile(ctx, "/app/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "file content" {
		t.Fatalf("expected 'file content', got %s", string(data))
	}

	// WriteFile (uses different runner for stdin pipe)
	w.runner = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockRunner("", "", 0)(ctx, name, arg...)
	}
	if err := w.WriteFile(ctx, "/app/out.txt", []byte("new data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// MkdirAll
	if err := w.MkdirAll(ctx, "/app/newdir", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
}

func TestK8sWorkspace_ListDir(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("file1.txt\ndir1/\nscript.sh*\n", "", 0),
	}

	entries, err := w.ListDir(context.Background(), "/app")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Name != "file1.txt" || entries[0].IsDir {
		t.Fatalf("entry 0: %+v", entries[0])
	}
	if entries[1].Name != "dir1" || !entries[1].IsDir {
		t.Fatalf("entry 1: %+v", entries[1])
	}
}

func TestK8sWorkspace_Stat(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("/app/test.txt|1024|regular empty file|1700000000", "", 0),
	}

	info, err := w.Stat(context.Background(), "/app/test.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name != "/app/test.txt" || info.Size != 1024 || info.IsDir {
		t.Fatalf("unexpected stat: %+v", info)
	}
}

func TestK8sWorkspace_StatDirectory(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("/app|0|directory|1700000000", "", 0),
	}

	info, err := w.Stat(context.Background(), "/app")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir {
		t.Fatal("should be directory")
	}
}

func TestK8sWorkspace_Execute(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("hello world", "some stderr", 0),
	}

	result, err := w.Execute(context.Background(), "echo hello", ExecuteOptions{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Stdout != "hello world" {
		t.Fatalf("expected 'hello world', got %s", result.Stdout)
	}
}

func TestK8sWorkspace_ExecuteWithWorkingDir(t *testing.T) {
	var capturedCmd string
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			for _, a := range arg {
				if strings.Contains(a, "cd ") {
					capturedCmd = a
				}
			}
			return mockRunner("ok", "", 0)(ctx, name, arg...)
		},
	}

	result, err := w.Execute(context.Background(), "ls", ExecuteOptions{WorkingDir: "/workspace"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code: %d", result.ExitCode)
	}
	if !strings.Contains(capturedCmd, "cd /workspace") {
		t.Fatalf("should cd to working dir, captured: %s", capturedCmd)
	}
}

func TestK8sWorkspace_ExecuteError(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("", "command not found", 127),
	}

	result, err := w.Execute(context.Background(), "nonexistent", ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 127 {
		t.Fatalf("expected exit code 127, got %d", result.ExitCode)
	}
}

func TestK8sWorkspace_Close(t *testing.T) {
	w := &K8sWorkspace{
		cfg:     K8sConfig{ID: "k1", Namespace: "default"},
		podName: "test-pod",
		runner:  mockRunner("pod deleted", "", 0),
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if w.podName != "" {
		t.Fatal("podName should be cleared after close")
	}
}

func TestK8sWorkspace_NewForExistingPod(t *testing.T) {
	w := NewK8sWorkspaceForExistingPod("k1", "myns", "existing-pod")
	if w.ID() != "k1" || w.Type() != "k8s" {
		t.Fatalf("unexpected workspace: %+v", w)
	}
	if w.podName != "existing-pod" {
		t.Fatalf("expected podName 'existing-pod', got %s", w.podName)
	}
	if w.cfg.Namespace != "myns" {
		t.Fatalf("expected namespace 'myns', got %s", w.cfg.Namespace)
	}
}

func TestK8sWorkspace_MissingImage(t *testing.T) {
	_, err := NewK8sWorkspace(context.Background(), K8sConfig{ID: "k1"})
	if err == nil {
		t.Fatal("should error without Image")
	}
}

func TestK8sWorkspace_FmtPerm(t *testing.T) {
	if fmtPerm(0o644) != "644" {
		t.Fatalf("expected 644, got %s", fmtPerm(0o644))
	}
	if fmtPerm(0o755) != "755" {
		t.Fatalf("expected 755, got %s", fmtPerm(0o755))
	}
}
