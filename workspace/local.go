package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// LocalWorkspace operates on the local host filesystem.
type LocalWorkspace struct {
	id      string
	baseDir string
}

// NewLocalWorkspace creates a LocalWorkspace.
func NewLocalWorkspace(id, baseDir string) *LocalWorkspace {
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
	}
	return &LocalWorkspace{id: id, baseDir: baseDir}
}

func (w *LocalWorkspace) ID() string   { return w.id }
func (w *LocalWorkspace) Type() string { return "local" }

func (w *LocalWorkspace) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if w.baseDir != "" {
		return filepath.Join(w.baseDir, path)
	}
	return path
}

func (w *LocalWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(w.resolve(path))
}

func (w *LocalWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	p := w.resolve(path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, perm)
}

func (w *LocalWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
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

func (w *LocalWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return os.MkdirAll(w.resolve(path), perm)
}

func (w *LocalWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	info, err := os.Stat(w.resolve(path))
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

func (w *LocalWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	} else if w.baseDir != "" {
		cmd.Dir = w.baseDir
	}

	if len(opts.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range opts.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	} else {
		cmd.Env = os.Environ()
	}

	stdout, err := cmd.Output()
	var stderr []byte
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		} else if ctx.Err() == context.DeadlineExceeded {
			return &ExecuteResult{
				ExitCode: -1,
				Stdout:   string(stdout),
				Stderr:   fmt.Sprintf("TimeoutError: command exceeded timeout of %v", opts.Timeout),
			}, nil
		} else {
			stderr = []byte(err.Error())
		}
	}

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil && exitCode == 0 {
		exitCode = -1
	}

	return &ExecuteResult{
		ExitCode: exitCode,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	}, nil
}

func (w *LocalWorkspace) Close() error { return nil }
