package workspace

import (
	"testing"
)

// Compile-time interface checks.
func TestWorkspace_InterfaceCompliance(t *testing.T) {
	var _ Workspace = (*LocalWorkspace)(nil)
	var _ Workspace = (*DockerWorkspace)(nil)
	var _ Workspace = (*E2BWorkspace)(nil)
}

func TestFileInfo(t *testing.T) {
	fi := FileInfo{Name: "test.txt", Size: 100, IsDir: false}
	if fi.Name != "test.txt" {
		t.Fatal("name mismatch")
	}
	if fi.IsDir {
		t.Fatal("expected file")
	}
}

func TestDirEntry(t *testing.T) {
	de := DirEntry{Name: "dir", IsDir: true}
	if de.Name != "dir" {
		t.Fatal("name mismatch")
	}
	if !de.IsDir {
		t.Fatal("expected directory")
	}
}

func TestExecuteResult(t *testing.T) {
	r := &ExecuteResult{ExitCode: 0, Stdout: "ok", Stderr: ""}
	if r.ExitCode != 0 {
		t.Fatal("exit code mismatch")
	}
}
