package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"
)

// OpenSandboxConfig configures an OpenSandboxWorkspace.
type OpenSandboxConfig struct {
	ID        string
	ServerURL string // OpenSandbox API base
	APIKey    string
	Image     string // base image (e.g. "python:3.12")
}

// OpenSandboxWorkspace runs commands and file ops via an OpenSandbox remote sandbox.
// Aligned with Python agentscope's OpenSandboxWorkspace (#15b5243e).
type OpenSandboxWorkspace struct {
	cfg       OpenSandboxConfig
	client    *http.Client
	sandboxID string
	apiBase   string // per-sandbox API base URL
}

// NewOpenSandboxWorkspace creates and provisions an OpenSandbox remote sandbox.
func NewOpenSandboxWorkspace(ctx context.Context, cfg OpenSandboxConfig) (*OpenSandboxWorkspace, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("opensandbox: ServerURL is required")
	}
	w := &OpenSandboxWorkspace{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
	if err := w.create(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *OpenSandboxWorkspace) ID() string   { return w.cfg.ID }
func (w *OpenSandboxWorkspace) Type() string { return "opensandbox" }

func (w *OpenSandboxWorkspace) create(ctx context.Context) error {
	body := map[string]any{}
	if w.cfg.Image != "" {
		body["image"] = w.cfg.Image
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", w.cfg.ServerURL+"/sandboxes", bytes.NewReader(data))
	if err != nil {
		return err
	}
	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("opensandbox: create: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensandbox: create: %s: %s", resp.Status, string(b))
	}
	var result struct {
		ID     string `json:"id"`
		APIURL string `json:"api_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("opensandbox: decode create: %w", err)
	}
	w.sandboxID = result.ID
	w.apiBase = result.APIURL
	if w.apiBase == "" {
		w.apiBase = w.cfg.ServerURL + "/sandboxes/" + result.ID
	}
	return nil
}

func (w *OpenSandboxWorkspace) Close() error {
	if w.sandboxID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "DELETE", w.cfg.ServerURL+"/sandboxes/"+w.sandboxID, nil)
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("opensandbox: delete: %w", err)
	}
	resp.Body.Close()
	w.sandboxID = ""
	return nil
}

func (w *OpenSandboxWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", w.apiBase+"/files?path="+path, nil)
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opensandbox ReadFile: %s: %s", resp.Status, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (w *OpenSandboxWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	body, _ := json.Marshal(map[string]any{"path": path, "content": string(data), "mode": uint32(perm)})
	req, _ := http.NewRequestWithContext(ctx, "PUT", w.apiBase+"/files", bytes.NewReader(body))
	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (w *OpenSandboxWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", w.apiBase+"/files/list?path="+path, nil)
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var entries []struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
	}
	json.NewDecoder(resp.Body).Decode(&entries)
	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = DirEntry{Name: e.Name, IsDir: e.IsDir}
	}
	return result, nil
}

func (w *OpenSandboxWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	body, _ := json.Marshal(map[string]any{"path": path})
	req, _ := http.NewRequestWithContext(ctx, "POST", w.apiBase+"/files/mkdir", bytes.NewReader(body))
	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (w *OpenSandboxWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", w.apiBase+"/files/stat?path="+path, nil)
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return FileInfo{}, err
	}
	defer resp.Body.Close()
	var info struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"is_dir"`
		ModTime int64  `json:"mod_time"`
	}
	json.NewDecoder(resp.Body).Decode(&info)
	return FileInfo{Name: info.Name, Size: info.Size, IsDir: info.IsDir, ModTime: time.Unix(info.ModTime, 0)}, nil
}

func (w *OpenSandboxWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	body, _ := json.Marshal(map[string]any{"command": command, "cwd": opts.WorkingDir})
	req, _ := http.NewRequestWithContext(ctx, "POST", w.apiBase+"/execute", bytes.NewReader(body))
	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		ExitCode int    `json:"exit_code"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return &ExecuteResult{ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr}, nil
}

func (w *OpenSandboxWorkspace) setHeaders(req *http.Request) {
	if w.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.cfg.APIKey)
	}
	req.Header.Set("Accept", "application/json")
}

var _ Workspace = (*OpenSandboxWorkspace)(nil)
