// Package workspace provides an abstraction over execution environments
// (local filesystem, Docker container, E2B sandbox) so that tools operate
// on the workspace rather than the host OS directly.
package workspace

import (
	"context"
	"io/fs"
	"time"
)

// Workspace is the execution environment abstraction.
type Workspace interface {
	// Identity
	ID() string
	Type() string // "local" | "docker" | "e2b"

	// File system operations (used by file tools)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error
	ListDir(ctx context.Context, path string) ([]DirEntry, error)
	MkdirAll(ctx context.Context, path string, perm fs.FileMode) error
	Stat(ctx context.Context, path string) (FileInfo, error)

	// Execution (used by shell tool)
	Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error)

	// Lifecycle
	Close() error
}

// FileInfo describes a file or directory within a workspace.
type FileInfo struct {
	Name    string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
	IsDir   bool
}

// DirEntry describes a single directory entry.
type DirEntry struct {
	Name  string
	IsDir bool
}

// ExecuteOptions controls command execution.
type ExecuteOptions struct {
	WorkingDir string
	Timeout    time.Duration
	Env        map[string]string
}

// ExecuteResult is the outcome of a command execution.
type ExecuteResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
