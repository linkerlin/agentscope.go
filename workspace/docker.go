package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// cmdRunner abstracts os/exec for testability.
type cmdRunner func(ctx context.Context, name string, arg ...string) *exec.Cmd

var defaultRunner cmdRunner = exec.CommandContext

// DockerWorkspace runs file and execution operations inside a Docker container
// using the docker CLI. No Go Docker SDK dependency required.
type DockerWorkspace struct {
	id          string
	containerID string
	cfg         DockerConfig
	runner      cmdRunner
}

// DockerConfig holds optional configuration for DockerWorkspace.
type DockerConfig struct {
	Image      string            // Docker image to use
	WorkDir    string            // Working directory inside container
	Env        map[string]string // Environment variables
	AutoRemove bool              // Remove container on close
	Privileged bool              // Run in privileged mode
}

// NewDockerWorkspace creates a DockerWorkspace that targets an existing container.
func NewDockerWorkspace(id, containerID string) *DockerWorkspace {
	return &DockerWorkspace{
		id:          id,
		containerID: containerID,
		runner:      defaultRunner,
	}
}

// NewDockerWorkspaceWithConfig creates a DockerWorkspace with configuration.
func NewDockerWorkspaceWithConfig(id, containerID string, cfg DockerConfig) *DockerWorkspace {
	return &DockerWorkspace{
		id:          id,
		containerID: containerID,
		cfg:         cfg,
		runner:      defaultRunner,
	}
}

func (w *DockerWorkspace) ID() string   { return w.id }
func (w *DockerWorkspace) Type() string { return "docker" }

// ReadFile copies a file from the container using docker cp.
func (w *DockerWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "docker-ws-read-*")
	if err != nil {
		return nil, fmt.Errorf("docker workspace: create temp: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cmd := w.runner(ctx, "docker", "cp", w.containerID+":"+path, tmpFile.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("docker workspace: ReadFile %s: %w\n%s", path, err, string(out))
	}
	return os.ReadFile(tmpFile.Name())
}

// WriteFile copies data into the container using docker cp.
func (w *DockerWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	tmpFile, err := os.CreateTemp("", "docker-ws-write-*")
	if err != nil {
		return fmt.Errorf("docker workspace: create temp: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("docker workspace: write temp: %w", err)
	}
	tmpFile.Close()
	if perm != 0 {
		if err := os.Chmod(tmpFile.Name(), perm); err != nil {
			return fmt.Errorf("docker workspace: chmod temp: %w", err)
		}
	}

	// Ensure parent directory exists.
	if err := w.MkdirAll(ctx, filepath.Dir(path), 0755); err != nil {
		// Ignore error if parent is root or already exists.
	}

	cmd := w.runner(ctx, "docker", "cp", tmpFile.Name(), w.containerID+":"+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker workspace: WriteFile %s: %w\n%s", path, err, string(out))
	}
	return nil
}

// ListDir runs ls -1 inside the container.
func (w *DockerWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	cmd := w.runner(ctx, "docker", "exec", w.containerID, "ls", "-1", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker workspace: ListDir %s: %w\n%s", path, err, string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	entries := make([]DirEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		entries = append(entries, DirEntry{Name: line})
	}
	return entries, nil
}

// MkdirAll runs mkdir -p inside the container.
func (w *DockerWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	args := []string{"exec", w.containerID, "mkdir", "-p"}
	if perm != 0 {
		args = append(args, "-m", fmt.Sprintf("%o", perm))
	}
	args = append(args, path)
	cmd := w.runner(ctx, "docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker workspace: MkdirAll %s: %w\n%s", path, err, string(out))
	}
	return nil
}

// Stat runs stat inside the container and parses the output.
func (w *DockerWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	// Use stat format: size|mtime|mode|name
	cmd := w.runner(ctx, "docker", "exec", w.containerID, "stat", "-c", "%s|%Y|%f|%n", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return FileInfo{}, fmt.Errorf("docker workspace: Stat %s: %w\n%s", path, err, string(out))
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) != 4 {
		return FileInfo{}, fmt.Errorf("docker workspace: Stat %s: unexpected output: %s", path, string(out))
	}
	size, _ := strconv.ParseInt(parts[0], 10, 64)
	mtimeSec, _ := strconv.ParseInt(parts[1], 10, 64)
	modeHex, _ := strconv.ParseUint(parts[2], 16, 32)
	name := parts[3]
	return FileInfo{
		Name:    filepath.Base(name),
		Size:    size,
		ModTime: time.Unix(mtimeSec, 0),
		Mode:    fs.FileMode(modeHex),
		IsDir:   fs.FileMode(modeHex).IsDir(),
	}, nil
}

// Execute runs a command inside the container using docker exec.
func (w *DockerWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	args := []string{"exec"}
	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", k+"="+v)
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	args = append(args, w.containerID, "sh", "-c", command)

	cmd := w.runner(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		err = nil // non-zero exit is still a successful execution from workspace perspective //nolint:ineffassign // intentional per comment
	}
	if ctx.Err() == context.DeadlineExceeded {
		return &ExecuteResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: -1,
		}, fmt.Errorf("docker workspace: Execute timeout: %w", ctx.Err())
	}
	return &ExecuteResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// Close optionally stops/removes the container if AutoRemove is set.
func (w *DockerWorkspace) Close() error {
	if w.cfg.AutoRemove {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := w.runner(ctx, "docker", "rm", "-f", w.containerID)
		_ = cmd.Run()
	}
	return nil
}

// CreateContainer creates a new container from an image and returns its ID.
// The container is started in detached mode.
func CreateContainer(ctx context.Context, image string, cfg DockerConfig) (string, error) {
	args := []string{"run", "-d", "--rm"}
	if cfg.Privileged {
		args = append(args, "--privileged")
	}
	for k, v := range cfg.Env {
		args = append(args, "-e", k+"="+v)
	}
	if cfg.WorkDir != "" {
		args = append(args, "-w", cfg.WorkDir)
	}
	args = append(args, image, "tail", "-f", "/dev/null")

	cmd := defaultRunner(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker workspace: create container: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

const defaultDockerBaseImage = "python:3.11-slim"

// DockerBuildConfig controls Dockerfile rendering for agent workspaces.
// Extended for more flavors to match Python reference (pypi install, src install, node support etc.).
type DockerBuildConfig struct {
	BaseImage                string
	WorkDir                  string
	PythonPackages           []string
	ExtraRunLines            []string
	Flavor                   string // "default", "pypi", "src", "node", "full"
	InstallAgentscopeFromSrc bool   // for src flavor
}

// RenderDockerfile renders a workspace Dockerfile from config.
// Supports multiple flavors for better parity with Python reference (pypi, src, node variants).
func RenderDockerfile(cfg DockerBuildConfig) string {
	base := cfg.BaseImage
	if base == "" {
		base = defaultDockerBaseImage
	}
	workdir := cfg.WorkDir
	if workdir == "" {
		workdir = "/workspace"
	}

	flavor := cfg.Flavor
	if flavor == "" {
		flavor = "default"
	}

	var tmpl *template.Template
	var data map[string]any

	switch flavor {
	case "pypi":
		tmpl = dockerfilePypiTemplate
		data = map[string]any{
			"BaseImage": base,
			"WorkDir":   workdir,
		}
	case "src":
		tmpl = dockerfileSrcTemplate
		data = map[string]any{
			"BaseImage": base,
			"WorkDir":   workdir,
		}
	case "node":
		tmpl = dockerfileNodeTemplate
		data = map[string]any{
			"BaseImage": base,
			"WorkDir":   workdir,
		}
	case "full":
		tmpl = dockerfileFullTemplate
		pkgs := cfg.PythonPackages
		if len(pkgs) == 0 {
			pkgs = []string{"agentscope"}
		}
		data = map[string]any{
			"BaseImage":      base,
			"WorkDir":        workdir,
			"PythonPackages": strings.Join(pkgs, " "),
			"ExtraRunLines":  cfg.ExtraRunLines,
		}
	default:
		tmpl = dockerfileTemplate
		installLines := []string{}
		if len(cfg.PythonPackages) > 0 {
			installLines = append(installLines,
				"RUN pip install --no-cache-dir "+strings.Join(cfg.PythonPackages, " "),
			)
		}
		installLines = append(installLines, cfg.ExtraRunLines...)
		data = map[string]any{
			"BaseImage":    base,
			"WorkDir":      workdir,
			"InstallLines": installLines,
		}
	}

	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, data)
	return strings.TrimSpace(buf.String())
}

var dockerfileTemplate = template.Must(template.New("dockerfile").Parse(`# syntax=docker/dockerfile:1
FROM {{.BaseImage}}
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR {{.WorkDir}}
{{range .InstallLines}}{{.}}
{{end}}
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD test -d {{.WorkDir}} || exit 1
`))

// pypi flavor: optimized for installing agentscope from PyPI + common deps
var dockerfilePypiTemplate = template.Must(template.New("dockerfile-pypi").Parse(`# syntax=docker/dockerfile:1
FROM {{.BaseImage}}
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
WORKDIR {{.WorkDir}}
RUN pip install --no-cache-dir agentscope[full]
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD test -d {{.WorkDir}} || exit 1
`))

// src flavor: for development / installing from source (editable or git)
var dockerfileSrcTemplate = template.Must(template.New("dockerfile-src").Parse(`# syntax=docker/dockerfile:1
FROM {{.BaseImage}}
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates git build-essential \
    && rm -rf /var/lib/apt/lists/*
WORKDIR {{.WorkDir}}
COPY . /install-src
RUN pip install --no-cache-dir -e /install-src[full] || pip install --no-cache-dir /install-src
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD test -d {{.WorkDir}} || exit 1
`))

// node flavor: adds Node.js for frontend/tooling use cases (e.g. browser tools, UI)
var dockerfileNodeTemplate = template.Must(template.New("dockerfile-node").Parse(`# syntax=docker/dockerfile:1
FROM {{.BaseImage}}
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates gnupg \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*
WORKDIR {{.WorkDir}}
RUN pip install --no-cache-dir agentscope[full]
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD test -d {{.WorkDir}} || exit 1
`))

// full flavor: combines common packages + extra lines (very flexible)
var dockerfileFullTemplate = template.Must(template.New("dockerfile-full").Parse(`# syntax=docker/dockerfile:1
FROM {{.BaseImage}}
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
WORKDIR {{.WorkDir}}
RUN pip install --no-cache-dir {{.PythonPackages}}
{{range .ExtraRunLines}}{{.}}
{{end}}
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD test -d {{.WorkDir}} || exit 1
`))

// ComputeImageTag returns a deterministic local image tag from Dockerfile content.
func ComputeImageTag(dockerfile string, extra ...[]byte) string {
	h := sha256.New()
	h.Write([]byte(dockerfile))
	for _, b := range extra {
		h.Write(b)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if len(sum) > 12 {
		sum = sum[:12]
	}
	return "agentscope-workspace:" + sum
}

// HealthCheck verifies a container is running (or healthy if configured).
func HealthCheck(ctx context.Context, containerID string) error {
	return HealthCheckWithRunner(ctx, containerID, defaultRunner)
}

// HealthCheckWithRunner allows injecting a command runner (for tests).
func HealthCheckWithRunner(ctx context.Context, containerID string, runner cmdRunner) error {
	cmd := runner(ctx, "docker", "inspect", "-f", "{{.State.Running}}", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker health inspect: %w\n%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("docker container %s is not running", containerID)
	}
	cmd = runner(ctx, "docker", "inspect", "-f", "{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}", containerID)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker health status: %w\n%s", err, string(out))
	}
	status := strings.TrimSpace(string(out))
	if status == "unhealthy" {
		return fmt.Errorf("docker container %s is unhealthy", containerID)
	}
	return nil
}

// BuildImage builds a Docker image from rendered Dockerfile bytes.
func BuildImage(ctx context.Context, dockerfile, tag string) error {
	return BuildImageWithRunner(ctx, dockerfile, tag, defaultRunner)
}

// BuildImageWithRunner writes Dockerfile to a temp dir and runs docker build.
func BuildImageWithRunner(ctx context.Context, dockerfile, tag string, runner cmdRunner) error {
	dir, err := os.MkdirTemp("", "agentscope-docker-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return err
	}
	cmd := runner(ctx, "docker", "build", "-t", tag, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, string(out))
	}
	return nil
}
