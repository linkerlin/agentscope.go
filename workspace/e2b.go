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

// E2BClient is a lightweight HTTP client for the E2B REST API.
// It covers sandbox lifecycle management. File and command operations
// require an envd gRPC connection (not yet implemented).
type E2BClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewE2BClient creates an E2B API client.
func NewE2BClient(apiKey string) *E2BClient {
	return &E2BClient{
		apiKey:     apiKey,
		baseURL:    "https://api.e2b.app",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithBaseURL sets a custom API base URL (for self-hosted E2B).
func (c *E2BClient) WithBaseURL(url string) *E2BClient {
	c.baseURL = url
	return c
}

// WithHTTPClient sets a custom HTTP client.
func (c *E2BClient) WithHTTPClient(client *http.Client) *E2BClient {
	c.httpClient = client
	return c
}

// CreateSandbox creates a new sandbox from the given template.
func (c *E2BClient) CreateSandbox(ctx context.Context, templateID string, timeout time.Duration) (sandboxID string, err error) {
	body := map[string]any{"template_id": templateID}
	if timeout > 0 {
		body["timeout"] = int(timeout.Seconds())
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/sandboxes", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("e2b create sandbox: %s: %s", resp.Status, string(b))
	}

	var result struct {
		SandboxID string `json:"sandbox_id"`
		ClientID  string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("e2b create sandbox: decode: %w", err)
	}
	return result.SandboxID, nil
}

// DeleteSandbox kills and removes a sandbox.
func (c *E2BClient) DeleteSandbox(ctx context.Context, sandboxID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/sandboxes/"+sandboxID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("e2b delete sandbox: %s: %s", resp.Status, string(b))
	}
	return nil
}

// RefreshSandbox extends the sandbox timeout.
func (c *E2BClient) RefreshSandbox(ctx context.Context, sandboxID string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/sandboxes/"+sandboxID+"/refreshes", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("e2b refresh sandbox: %s: %s", resp.Status, string(b))
	}
	return nil
}

// SetSandboxTimeout changes the sandbox timeout.
func (c *E2BClient) SetSandboxTimeout(ctx context.Context, sandboxID string, timeout time.Duration) error {
	body := map[string]any{"timeout": int(timeout.Seconds())}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/sandboxes/"+sandboxID+"/timeout", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("e2b set timeout: %s: %s", resp.Status, string(b))
	}
	return nil
}

// ----------------------------------------------------------------------
// E2BWorkspace
// ----------------------------------------------------------------------

// E2BWorkspace runs file and execution operations inside an E2B cloud sandbox.
// Sandbox lifecycle is managed via the E2B REST API. File and command
// operations require an envd gRPC connection and are currently stubbed.
type E2BWorkspace struct {
	id        string
	sandboxID string
	client    *E2BClient
}

// NewE2BWorkspace creates an E2BWorkspace backed by an existing sandbox.
func NewE2BWorkspace(id, sandboxID string, client *E2BClient) *E2BWorkspace {
	return &E2BWorkspace{id: id, sandboxID: sandboxID, client: client}
}

// CreateE2BWorkspace creates a new E2B sandbox and wraps it as a Workspace.
func CreateE2BWorkspace(ctx context.Context, id, templateID string, client *E2BClient, timeout time.Duration) (*E2BWorkspace, error) {
	sandboxID, err := client.CreateSandbox(ctx, templateID, timeout)
	if err != nil {
		return nil, err
	}
	return &E2BWorkspace{id: id, sandboxID: sandboxID, client: client}, nil
}

func (w *E2BWorkspace) ID() string   { return w.id }
func (w *E2BWorkspace) Type() string { return "e2b" }

func (w *E2BWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, fmt.Errorf("e2b workspace: ReadFile requires envd gRPC (not yet implemented)")
}

func (w *E2BWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	return fmt.Errorf("e2b workspace: WriteFile requires envd gRPC (not yet implemented)")
}

func (w *E2BWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	return nil, fmt.Errorf("e2b workspace: ListDir requires envd gRPC (not yet implemented)")
}

func (w *E2BWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return fmt.Errorf("e2b workspace: MkdirAll requires envd gRPC (not yet implemented)")
}

func (w *E2BWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	return FileInfo{}, fmt.Errorf("e2b workspace: Stat requires envd gRPC (not yet implemented)")
}

func (w *E2BWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	return nil, fmt.Errorf("e2b workspace: Execute requires envd gRPC (not yet implemented)")
}

// Close deletes the underlying E2B sandbox.
func (w *E2BWorkspace) Close() error {
	if w.client == nil || w.sandboxID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return w.client.DeleteSandbox(ctx, w.sandboxID)
}

// Refresh extends the sandbox timeout via the REST API.
func (w *E2BWorkspace) Refresh(ctx context.Context) error {
	if w.client == nil {
		return fmt.Errorf("e2b workspace: no client")
	}
	return w.client.RefreshSandbox(ctx, w.sandboxID)
}

// SandboxID returns the E2B sandbox ID.
func (w *E2BWorkspace) SandboxID() string { return w.sandboxID }
