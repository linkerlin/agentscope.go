package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// K8sConfig configures a K8sWorkspace.
type K8sConfig struct {
	// ID is the workspace identifier.
	ID string
	// Namespace is the K8s namespace (default: "default").
	Namespace string
	// Image is the container image for the Pod.
	Image string
	// PodName overrides the auto-generated pod name.
	PodName string
	// Labels are extra K8s labels for the Pod.
	Labels map[string]string
	// Command is the Pod's entrypoint (default: sleep infinity).
	Command []string
	// Kubeconfig path (optional; uses KUBECONFIG env or ~/.kube/config by default).
	Kubeconfig string
}

// K8sWorkspace runs file and execution operations inside a Kubernetes Pod
// using the kubectl CLI. No client-go dependency required — keeps the binary
// small and leverages the operator's existing kubeconfig.
//
// Aligned with Python agentscope's K8sWorkspace (#81538d35).
type K8sWorkspace struct {
	cfg     K8sConfig
	podName string
	runner  cmdRunner
}

// NewK8sWorkspace creates a K8sWorkspace and provisions a Pod.
func NewK8sWorkspace(ctx context.Context, cfg K8sConfig) (*K8sWorkspace, error) {
	if cfg.Image == "" {
		return nil, fmt.Errorf("k8s: Image is required")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	w := &K8sWorkspace{cfg: cfg, runner: defaultRunner}
	if err := w.createPod(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

// NewK8sWorkspaceForExistingPod wraps an already-running Pod.
func NewK8sWorkspaceForExistingPod(id, namespace, podName string) *K8sWorkspace {
	if namespace == "" {
		namespace = "default"
	}
	return &K8sWorkspace{
		cfg:     K8sConfig{ID: id, Namespace: namespace, PodName: podName},
		podName: podName,
		runner:  defaultRunner,
	}
}

func (w *K8sWorkspace) ID() string   { return w.cfg.ID }
func (w *K8sWorkspace) Type() string { return "k8s" }

// --- Pod lifecycle ---

func (w *K8sWorkspace) createPod(ctx context.Context) error {
	name := w.cfg.PodName
	if name == "" {
		name = "as-ws-" + w.cfg.ID
	}

	args := []string{"run", name, "--image=" + w.cfg.Image, "--restart=Never", "--namespace=" + w.cfg.Namespace}
	for k, v := range w.cfg.Labels {
		args = append(args, "--labels="+k+"="+v)
	}
	cmd := w.runner(ctx, "kubectl", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("k8s: create pod: %w: %s", err, string(out))
	}
	w.podName = name

	// Wait for Pod to be Ready.
	waitCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	waitCmd := w.runner(waitCtx, "kubectl", "wait", "--for=condition=Ready", "pod/"+name,
		"--namespace="+w.cfg.Namespace, "--timeout=120s")
	if out, err := waitCmd.CombinedOutput(); err != nil {
		// Clean up the pod if wait fails.
		_ = w.deletePod(context.Background())
		return fmt.Errorf("k8s: wait pod ready: %w: %s", err, string(out))
	}
	return nil
}

func (w *K8sWorkspace) deletePod(ctx context.Context) error {
	if w.podName == "" {
		return nil
	}
	cmd := w.runner(ctx, "kubectl", "delete", "pod", w.podName,
		"--namespace="+w.cfg.Namespace, "--ignore-not-found=true", "--force")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("k8s: delete pod: %w: %s", err, string(out))
	}
	w.podName = ""
	return nil
}

func (w *K8sWorkspace) Close() error {
	return w.deletePod(context.Background())
}

// --- File operations (via kubectl exec) ---

func (w *K8sWorkspace) execPod(ctx context.Context, args ...string) (string, string, int, error) {
	full := append([]string{"exec", w.podName, "--namespace=" + w.cfg.Namespace, "--"}, args...)
	cmd := w.runner(ctx, "kubectl", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", "", -1, err
		}
	}
	return stdout.String(), stderr.String(), exitCode, nil
}

func (w *K8sWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	stdout, stderr, code, err := w.execPod(ctx, "cat", path)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("k8s ReadFile %s: exit %d: %s", path, code, stderr)
	}
	return []byte(stdout), nil
}

func (w *K8sWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	// Ensure parent dir exists
	dir := path
	if idx := strings.LastIndex(path, "/"); idx > 0 {
		dir = path[:idx]
		w.execPod(ctx, "mkdir", "-p", dir)
	}
	// Write via stdin pipe to tee
	full := append([]string{"exec", w.podName, "--namespace=" + w.cfg.Namespace, "-i", "--"},
		"sh", "-c", fmt.Sprintf("cat > %s && chmod %s %s", path, fmtPerm(perm), path))
	cmd := w.runner(ctx, "kubectl", full...)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("k8s WriteFile %s: %w: %s", path, err, stderr.String())
	}
	return nil
}

func (w *K8sWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	stdout, stderr, code, err := w.execPod(ctx, "ls", "-1A", "--file-type", path)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("k8s ListDir %s: exit %d: %s", path, code, stderr)
	}
	var entries []DirEntry
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		isDir := strings.HasSuffix(line, "/")
		name := strings.TrimRight(line, "/*@=|/")
		entries = append(entries, DirEntry{Name: name, IsDir: isDir})
	}
	return entries, nil
}

func (w *K8sWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	_, _, code, err := w.execPod(ctx, "mkdir", "-p", path)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("k8s MkdirAll %s: exit %d", path, code)
	}
	return nil
}

func (w *K8sWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	stdout, stderr, code, err := w.execPod(ctx, "stat", "-c", "%n|%s|%F|%Y", path)
	if err != nil {
		return FileInfo{}, err
	}
	if code != 0 {
		return FileInfo{}, fmt.Errorf("k8s Stat %s: exit %d: %s", path, code, stderr)
	}
	parts := strings.SplitN(strings.TrimSpace(stdout), "|", 4)
	if len(parts) < 4 {
		return FileInfo{}, fmt.Errorf("k8s Stat: unexpected output: %s", stdout)
	}
	size, _ := strconv.ParseInt(parts[1], 10, 64)
	modUnix, _ := strconv.ParseInt(parts[3], 10, 64)
	return FileInfo{
		Name:    parts[0],
		Size:    size,
		IsDir:   strings.Contains(parts[2], "directory"),
		ModTime: time.Unix(modUnix, 0),
	}, nil
}

func (w *K8sWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	shellCmd := command
	if opts.WorkingDir != "" {
		shellCmd = fmt.Sprintf("cd %s && %s", opts.WorkingDir, command)
	}
	args := []string{"exec", "-i", w.podName, "--namespace=" + w.cfg.Namespace, "--", "sh", "-c", shellCmd}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	cmd := w.runner(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("k8s execute: %w", err)
		}
	}
	return &ExecuteResult{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

// fmtPerm converts fs.FileMode to octal string (e.g. "644").
func fmtPerm(perm fs.FileMode) string {
	return strconv.FormatUint(uint64(perm&0o777), 8)
}

var _ Workspace = (*K8sWorkspace)(nil)
