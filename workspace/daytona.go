package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// DaytonaConfig configures a DaytonaWorkspace.
type DaytonaConfig struct {
	// ID is the workspace identifier.
	ID string
	// ServerURL is the Daytona server API base (e.g. "https://api.daytona.io").
	ServerURL string
	// APIKey is the Daytona API key.
	APIKey string
	// Target is the workspace target (e.g. "local", "aws").
	Target string
	// Image is the base container image for the workspace.
	Image string
}

// DaytonaWorkspace runs commands and file ops via a Daytona remote dev environment.
// Aligned with Python agentscope's DaytonaWorkspace (#dd71a372).
type DaytonaWorkspace struct {
	cfg         DaytonaConfig
	client      *http.Client
	workspaceID string
	nodeURL     string // per-workspace API endpoint
}

// NewDaytonaWorkspace creates a DaytonaWorkspace and provisions a remote environment.
func NewDaytonaWorkspace(ctx context.Context, cfg DaytonaConfig) (*DaytonaWorkspace, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("daytona: ServerURL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("daytona: APIKey is required")
	}
	w := &DaytonaWorkspace{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
	if err := w.create(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *DaytonaWorkspace) ID() string   { return w.cfg.ID }
func (w *DaytonaWorkspace) Type() string { return "daytona" }

// --- Lifecycle ---

func (w *DaytonaWorkspace) create(ctx context.Context) error {
	body := map[string]any{}
	if w.cfg.Image != "" {
		body["image"] = w.cfg.Image
	}
	if w.cfg.Target != "" {
		body["target"] = w.cfg.Target
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", w.cfg.ServerURL+"/workspace", bytes.NewReader(data))
	if err != nil {
		return err
	}
	w.setHeaders(req)

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("daytona: create workspace: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daytona: create workspace: %s: %s", resp.Status, string(b))
	}
	var result struct {
		ID      string `json:"id"`
		NodeURL string `json:"nodeUrl,omitempty"`
		Info    struct {
			NodeURL string `json:"nodeUrl,omitempty"`
		} `json:"info,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("daytona: decode create response: %w", err)
	}
	w.workspaceID = result.ID
	w.nodeURL = result.NodeURL
	if w.nodeURL == "" {
		w.nodeURL = result.Info.NodeURL
	}
	if w.nodeURL == "" {
		w.nodeURL = w.cfg.ServerURL
	}
	return nil
}

func (w *DaytonaWorkspace) Close() error {
	if w.workspaceID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		w.cfg.ServerURL+"/workspace/"+w.workspaceID, nil)
	if err != nil {
		return err
	}
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("daytona: delete workspace: %w", err)
	}
	resp.Body.Close()
	w.workspaceID = ""
	return nil
}

// --- File operations (via Daytona toolbox API) ---

func (w *DaytonaWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		w.nodeURL+"/toolbox/"+w.workspaceID+"/file?path="+path, nil)
	if err != nil {
		return nil, err
	}
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daytona ReadFile: %s: %s", resp.Status, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (w *DaytonaWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	body := map[string]any{
		"path":    path,
		"content": base64.StdEncoding.EncodeToString(data),
		"mode":    uint32(perm),
	}
	dataJSON, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		w.nodeURL+"/toolbox/"+w.workspaceID+"/file", bytes.NewReader(dataJSON))
	if err != nil {
		return err
	}
	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daytona WriteFile: %s: %s", resp.Status, string(b))
	}
	return nil
}

func (w *DaytonaWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		w.nodeURL+"/toolbox/"+w.workspaceID+"/dir?path="+path, nil)
	if err != nil {
		return nil, err
	}
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daytona ListDir: %s: %s", resp.Status, string(b))
	}
	var entries []struct {
		Name  string `json:"name"`
		IsDir bool   `json:"isDir"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = DirEntry{Name: e.Name, IsDir: e.IsDir}
	}
	return result, nil
}

func (w *DaytonaWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	body := map[string]any{"path": path, "mode": uint32(perm)}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		w.nodeURL+"/toolbox/"+w.workspaceID+"/mkdir", bytes.NewReader(data))
	if err != nil {
		return err
	}
	w.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (w *DaytonaWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		w.nodeURL+"/toolbox/"+w.workspaceID+"/stat?path="+path, nil)
	if err != nil {
		return FileInfo{}, err
	}
	w.setHeaders(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return FileInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return FileInfo{}, fmt.Errorf("daytona Stat: %s: %s", resp.Status, string(b))
	}
	var info struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"isDir"`
		ModTime int64  `json:"modTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name: info.Name, Size: info.Size, IsDir: info.IsDir,
		ModTime: time.Unix(info.ModTime, 0),
	}, nil
}

// --- Execute ---

func (w *DaytonaWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	body := map[string]any{
		"command": command,
		"cwd":     opts.WorkingDir,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		w.nodeURL+"/toolbox/"+w.workspaceID+"/execute", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
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
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("daytona execute decode: %w", err)
	}
	return &ExecuteResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, nil
}

func (w *DaytonaWorkspace) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+w.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
}

// compile-time interface check
var _ Workspace = (*DaytonaWorkspace)(nil)

// suppress unused import on some build configs
var _ = strings.TrimSpace
