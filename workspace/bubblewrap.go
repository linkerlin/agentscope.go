package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// BubblewrapConfig configures a BubblewrapWorkspace.
type BubblewrapConfig struct {
	// ID is the workspace identifier.
	ID string
	// BaseDir is the host working directory bind-mounted into the sandbox.
	BaseDir string
	// ReadOnlyDirs are additional host dirs to bind-mount read-only.
	// Defaults to /usr, /lib, /bin, /sbin, /lib64, /etc/resolv.conf.
	ReadOnlyDirs map[string]string // hostPath → sandboxPath
	// TmpfsSize caps the /tmp tmpfs in bytes (0 = default).
	TmpfsSize int64
	// ShareNet keeps the host network namespace (default: unshare).
	ShareNet bool
}

// BubblewrapWorkspace runs commands inside a Linux Bubblewrap (bwrap)
// user-namespace sandbox. File operations (Read/Write/List) act directly on
// the host BaseDir (which is bind-mounted read-write into the sandbox);
// Execute spawns a fresh bwrap per command (stateless, simple).
//
// Aligned with Python agentscope's BubblewrapWorkspace (#7af58b11).
type BubblewrapWorkspace struct {
	cfg BubblewrapConfig
	abs string // absolute BaseDir
}

// NewBubblewrapWorkspace creates a BubblewrapWorkspace.
func NewBubblewrapWorkspace(cfg BubblewrapConfig) (*BubblewrapWorkspace, error) {
	if cfg.BaseDir == "" {
		return nil, fmt.Errorf("bubblewrap: BaseDir is required")
	}
	abs, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("bubblewrap: resolve BaseDir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("bubblewrap: mkdir BaseDir: %w", err)
	}
	return &BubblewrapWorkspace{cfg: cfg, abs: abs}, nil
}

func (w *BubblewrapWorkspace) ID() string   { return w.cfg.ID }
func (w *BubblewrapWorkspace) Type() string { return "bubblewrap" }

// resolve converts a sandbox-relative path to a host path.
func (w *BubblewrapWorkspace) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(w.abs, path)
}

// --- File operations: direct on host (BaseDir is bind-mounted into sandbox) ---

func (w *BubblewrapWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(w.resolve(path))
}

func (w *BubblewrapWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	p := w.resolve(path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, perm)
}

func (w *BubblewrapWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	entries, err := os.ReadDir(w.resolve(path))
	if err != nil {
		return nil, err
	}
	var result []DirEntry
	for _, e := range entries {
		result = append(result, DirEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return result, nil
}

func (w *BubblewrapWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return os.MkdirAll(w.resolve(path), perm)
}

func (w *BubblewrapWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	info, err := os.Stat(w.resolve(path))
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name: info.Name(), Size: info.Size(), Mode: info.Mode(),
		ModTime: info.ModTime(), IsDir: info.IsDir(),
	}, nil
}

// --- Execute: run command inside bwrap sandbox ---

func (w *BubblewrapWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("bubblewrap: only supported on Linux (current: %s)", runtime.GOOS)
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, fmt.Errorf("bubblewrap: bwrap not found in PATH: %w", err)
	}

	args := w.buildBwrapArgs(opts.WorkingDir)
	args = append(args, "sh", "-c", command)

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Dir = w.abs
	if len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("bubblewrap: exec: %w", err)
		}
	}
	return &ExecuteResult{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func (w *BubblewrapWorkspace) Close() error { return nil }

// buildBwrapArgs constructs the bwrap argument list for sandbox setup.
// Testable without running bwrap.
func (w *BubblewrapWorkspace) buildBwrapArgs(workingDir string) []string {
	roDirs := w.cfg.ReadOnlyDirs
	if roDirs == nil {
		roDirs = defaultReadOnlyDirs()
	}

	var args []string

	// Read-only system mounts
	for host, sandbox := range roDirs {
		if _, err := os.Stat(host); err == nil {
			args = append(args, "--ro-bind", host, sandbox)
		}
	}

	// Proc + dev
	args = append(args, "--proc", "/proc")
	args = append(args, "--dev", "/dev")

	// Tmpfs for /tmp
	if w.cfg.TmpfsSize > 0 {
		args = append(args, "--tmpfs", "/tmp", "--size", fmt.Sprintf("%d", w.cfg.TmpfsSize))
	} else {
		args = append(args, "--tmpfs", "/tmp")
	}

	// Bind-mount BaseDir read-write as /workspace
	args = append(args, "--bind", w.abs, "/workspace")

	// Namespace isolation
	if !w.cfg.ShareNet {
		args = append(args, "--unshare-net")
	}
	args = append(args, "--unshare-all", "--die-with-parent")

	// Working directory inside sandbox
	wd := "/workspace"
	if workingDir != "" {
		wd = path.Join("/workspace", workingDir)
	}
	args = append(args, "--chdir", wd)

	return args
}

// defaultReadOnlyDirs returns the standard Linux system directories
// to bind-mount read-only inside the sandbox.
func defaultReadOnlyDirs() map[string]string {
	return map[string]string{
		"/usr":             "/usr",
		"/lib":             "/lib",
		"/lib32":           "/lib32",
		"/lib64":           "/lib64",
		"/bin":             "/bin",
		"/sbin":            "/sbin",
		"/etc/resolv.conf": "/etc/resolv.conf",
		"/etc/ssl":         "/etc/ssl",
	}
}

// compile-time interface check
var _ Workspace = (*BubblewrapWorkspace)(nil)
