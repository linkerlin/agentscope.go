package workspace

import (
	"context"
	"fmt"
	"io/fs"
)

// DockerWorkspace runs file and execution operations inside a Docker container.
// This is a skeleton implementation; the full Docker client integration can be
// added later without changing the interface.
type DockerWorkspace struct {
	id          string
	containerID string
}

// NewDockerWorkspace creates a DockerWorkspace skeleton.
func NewDockerWorkspace(id, containerID string) *DockerWorkspace {
	return &DockerWorkspace{id: id, containerID: containerID}
}

func (w *DockerWorkspace) ID() string   { return w.id }
func (w *DockerWorkspace) Type() string { return "docker" }

func (w *DockerWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, fmt.Errorf("docker workspace: ReadFile not yet implemented")
}

func (w *DockerWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	return fmt.Errorf("docker workspace: WriteFile not yet implemented")
}

func (w *DockerWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	return nil, fmt.Errorf("docker workspace: ListDir not yet implemented")
}

func (w *DockerWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return fmt.Errorf("docker workspace: MkdirAll not yet implemented")
}

func (w *DockerWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	return FileInfo{}, fmt.Errorf("docker workspace: Stat not yet implemented")
}

func (w *DockerWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	return nil, fmt.Errorf("docker workspace: Execute not yet implemented")
}

func (w *DockerWorkspace) Close() error { return nil }


// DockerConfig holds optional configuration for DockerWorkspace.
type DockerConfig struct {
	Image       string            // Docker image to use
	WorkDir     string            // Working directory inside container
	Env         map[string]string // Environment variables
	AutoRemove  bool              // Remove container on close
	Privileged  bool              // Run in privileged mode
}

// NewDockerWorkspaceWithConfig creates a DockerWorkspace with configuration.
func NewDockerWorkspaceWithConfig(id, containerID string, cfg DockerConfig) *DockerWorkspace {
	return &DockerWorkspace{id: id, containerID: containerID}
}
