package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"time"
)

// E2BWorkspace runs file and execution operations inside an E2B cloud sandbox.
// This is a skeleton implementation; the full E2B client integration can be
// added later without changing the interface.
type E2BWorkspace struct {
	id        string
	sandboxID string
}

// NewE2BWorkspace creates an E2BWorkspace skeleton.
func NewE2BWorkspace(id, sandboxID string) *E2BWorkspace {
	return &E2BWorkspace{id: id, sandboxID: sandboxID}
}

func (w *E2BWorkspace) ID() string   { return w.id }
func (w *E2BWorkspace) Type() string { return "e2b" }

func (w *E2BWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, fmt.Errorf("e2b workspace: ReadFile not yet implemented")
}

func (w *E2BWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	return fmt.Errorf("e2b workspace: WriteFile not yet implemented")
}

func (w *E2BWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	return nil, fmt.Errorf("e2b workspace: ListDir not yet implemented")
}

func (w *E2BWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return fmt.Errorf("e2b workspace: MkdirAll not yet implemented")
}

func (w *E2BWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	return FileInfo{}, fmt.Errorf("e2b workspace: Stat not yet implemented")
}

func (w *E2BWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	return nil, fmt.Errorf("e2b workspace: Execute not yet implemented")
}

func (w *E2BWorkspace) Close() error { return nil }


// E2BConfig holds optional configuration for E2BWorkspace.
type E2BConfig struct {
	TemplateID string            // E2B sandbox template ID
	Timeout    time.Duration     // Sandbox timeout
	Env        map[string]string // Environment variables
}

// NewE2BWorkspaceWithConfig creates an E2BWorkspace with configuration.
func NewE2BWorkspaceWithConfig(id, sandboxID string, cfg E2BConfig) *E2BWorkspace {
	return &E2BWorkspace{id: id, sandboxID: sandboxID}
}
