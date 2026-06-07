package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// E2BClient is a lightweight HTTP client for the E2B REST API.
// It covers sandbox lifecycle management. File and command operations
// use the envd HTTP/Connect-RPC JSON API.
type E2BClient struct {
	apiKey     string
	baseURL    string
	domain     string
	httpClient *http.Client
}

// NewE2BClient creates an E2B API client.
func NewE2BClient(apiKey string) *E2BClient {
	return &E2BClient{
		apiKey:     apiKey,
		baseURL:    "https://api.e2b.app",
		domain:     "e2b.dev",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithBaseURL sets a custom API base URL (for self-hosted E2B).
func (c *E2BClient) WithBaseURL(url string) *E2BClient {
	c.baseURL = url
	return c
}

// WithDomain sets a custom envd domain (for self-hosted E2B).
func (c *E2BClient) WithDomain(domain string) *E2BClient {
	c.domain = domain
	return c
}

// WithHTTPClient sets a custom HTTP client.
func (c *E2BClient) WithHTTPClient(client *http.Client) *E2BClient {
	c.httpClient = client
	return c
}

// CreateSandbox creates a new sandbox from the given template.
// Returns the full sandbox ID in the format "sandbox_id-client_id".
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
	if result.ClientID != "" {
		return result.SandboxID + "-" + result.ClientID, nil
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

const envdDefaultPort = 49983
const envdDefaultUser = "user"

// E2BWorkspace runs file and execution operations inside an E2B cloud sandbox.
type E2BWorkspace struct {
	id        string
	sandboxID string
	client    *E2BClient
	envdURL   string
	envdHTTP  *http.Client
}

// NewE2BWorkspace creates an E2BWorkspace backed by an existing sandbox.
func NewE2BWorkspace(id, sandboxID string, client *E2BClient) *E2BWorkspace {
	w := &E2BWorkspace{id: id, sandboxID: sandboxID, client: client}
	w.initEnvd()
	return w
}

// CreateE2BWorkspace creates a new E2B sandbox and wraps it as a Workspace.
func CreateE2BWorkspace(ctx context.Context, id, templateID string, client *E2BClient, timeout time.Duration) (*E2BWorkspace, error) {
	sandboxID, err := client.CreateSandbox(ctx, templateID, timeout)
	if err != nil {
		return nil, err
	}
	w := &E2BWorkspace{id: id, sandboxID: sandboxID, client: client}
	w.initEnvd()
	return w, nil
}

// initEnvd constructs the envd URL and HTTP client.
func (w *E2BWorkspace) initEnvd() {
	if w.client == nil {
		return
	}
	domain := w.client.domain
	if domain == "" {
		domain = "e2b.dev"
	}
	w.envdURL = fmt.Sprintf("https://%d-%s.%s", envdDefaultPort, w.sandboxID, domain)
	w.envdHTTP = &http.Client{
		Timeout: 5 * time.Minute, // longer timeout for streaming operations
	}
}

func (w *E2BWorkspace) ID() string   { return w.id }
func (w *E2BWorkspace) Type() string { return "e2b" }

// envdAuthHeader returns the Basic auth header for envd requests.
func envdAuthHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(envdDefaultUser+":"))
}

// envdRequest creates an HTTP request for the envd API.
func (w *E2BWorkspace) envdRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(w.envdURL + path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", envdAuthHeader())
	return req, nil
}

// ----------------------------------------------------------------------
// File operations (HTTP REST)
// ----------------------------------------------------------------------

// ReadFile reads a file from the sandbox.
func (w *E2BWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if w.envdHTTP == nil {
		return nil, fmt.Errorf("e2b workspace: no envd client")
	}
	u, err := url.Parse(w.envdURL + "/files")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("path", path)
	q.Set("username", envdDefaultUser)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", envdAuthHeader())

	resp, err := w.envdHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("e2b read file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("e2b read file: %s: %s", resp.Status, string(b))
	}

	return io.ReadAll(resp.Body)
}

// WriteFile writes data to a file in the sandbox.
func (w *E2BWorkspace) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	if w.envdHTTP == nil {
		return fmt.Errorf("e2b workspace: no envd client")
	}
	u, err := url.Parse(w.envdURL + "/files")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("path", path)
	q.Set("username", envdDefaultUser)
	u.RawQuery = q.Encode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "file")
	if err != nil {
		return fmt.Errorf("e2b write file: create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("e2b write file: write data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("e2b write file: close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", envdAuthHeader())
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := w.envdHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("e2b write file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("e2b write file: %s: %s", resp.Status, string(b))
	}

	// The response contains file info; we ignore it for now.
	_, _ = io.ReadAll(resp.Body)
	return nil
}

// ----------------------------------------------------------------------
// Directory operations (Connect-RPC JSON)
// ----------------------------------------------------------------------

// entryInfo represents a filesystem entry from envd.
type entryInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// connectRPCError represents a Connect-RPC error response.
type connectRPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// doConnectRPC sends a Connect-RPC JSON request and unmarshals the response.
func (w *E2BWorkspace) doConnectRPC(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("e2b rpc: marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := w.envdRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if reqBody != nil {
		req.Header.Set("Connect-Protocol-Version", "1")
	}

	resp, err := w.envdHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("e2b rpc: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		var cerr connectRPCError
		if json.Unmarshal(b, &cerr) == nil && cerr.Message != "" {
			return fmt.Errorf("e2b rpc: %s: %s", cerr.Code, cerr.Message)
		}
		return fmt.Errorf("e2b rpc: %s: %s", resp.Status, string(b))
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("e2b rpc: decode response: %w", err)
		}
	}
	return nil
}

// fileTypeToIsDir maps envd FileType string to IsDir bool.
func fileTypeToIsDir(ft string) bool {
	return ft == "FILE_TYPE_DIRECTORY"
}

// ListDir lists entries in a directory.
func (w *E2BWorkspace) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	if w.envdHTTP == nil {
		return nil, fmt.Errorf("e2b workspace: no envd client")
	}
	var resp struct {
		Entries []entryInfo `json:"entries"`
	}
	if err := w.doConnectRPC(ctx, "ListDir", "/filesystem.Filesystem/ListDir", map[string]string{"path": path}, &resp); err != nil {
		return nil, err
	}

	entries := make([]DirEntry, len(resp.Entries))
	for i, e := range resp.Entries {
		entries[i] = DirEntry{
			Name:  e.Name,
			IsDir: fileTypeToIsDir(e.Type),
		}
	}
	return entries, nil
}

// Stat returns file info.
func (w *E2BWorkspace) Stat(ctx context.Context, path string) (FileInfo, error) {
	if w.envdHTTP == nil {
		return FileInfo{}, fmt.Errorf("e2b workspace: no envd client")
	}
	var resp struct {
		Entry entryInfo `json:"entry"`
	}
	if err := w.doConnectRPC(ctx, "Stat", "/filesystem.Filesystem/Stat", map[string]string{"path": path}, &resp); err != nil {
		if strings.Contains(err.Error(), "not_found") {
			return FileInfo{}, fs.ErrNotExist
		}
		return FileInfo{}, err
	}

	return FileInfo{
		Name:  resp.Entry.Name,
		IsDir: fileTypeToIsDir(resp.Entry.Type),
	}, nil
}

// MkdirAll creates a directory and all parents.
func (w *E2BWorkspace) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	if w.envdHTTP == nil {
		return fmt.Errorf("e2b workspace: no envd client")
	}
	err := w.doConnectRPC(ctx, "MakeDir", "/filesystem.Filesystem/MakeDir", map[string]string{"path": path}, nil)
	if err != nil {
		// already_exists is not an error for MkdirAll.
		if strings.Contains(err.Error(), "already_exists") {
			return nil
		}
		return err
	}
	return nil
}

// ----------------------------------------------------------------------
// Command execution (Connect-RPC JSON streaming)
// ----------------------------------------------------------------------

// processEvent represents a single event from the process stream.
type processEvent struct {
	Event struct {
		Start struct {
			PID uint32 `json:"pid"`
		} `json:"start"`
		Data struct {
			Stdout string `json:"stdout"`
			Stderr string `json:"stderr"`
		} `json:"data"`
		End struct {
			ExitCode int    `json:"exitCode"`
			Error    string `json:"error"`
		} `json:"end"`
	} `json:"event"`
}

// Execute runs a command inside the sandbox.
func (w *E2BWorkspace) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	if w.envdHTTP == nil {
		return nil, fmt.Errorf("e2b workspace: no envd client")
	}
	reqBody := map[string]any{
		"process": map[string]any{
			"cmd":  "/bin/bash",
			"args": []string{"-l", "-c", command},
		},
	}

	if opts.WorkingDir != "" {
		reqBody["process"].(map[string]any)["cwd"] = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		reqBody["process"].(map[string]any)["envs"] = opts.Env
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("e2b execute: marshal request: %w", err)
	}

	req, err := w.envdRequest(ctx, http.MethodPost, "/process.Process/Start", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Accept", "application/json")

	// Apply timeout if specified.
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := w.envdHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("e2b execute: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("e2b execute: %s: %s", resp.Status, string(b))
	}

	result := &ExecuteResult{ExitCode: -1}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var ev processEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip malformed lines
		}

		if ev.Event.Start.PID != 0 {
			// Process started; no action needed.
			continue
		}
		if ev.Event.Data.Stdout != "" {
			decoded, _ := base64.StdEncoding.DecodeString(ev.Event.Data.Stdout)
			result.Stdout += string(decoded)
		}
		if ev.Event.Data.Stderr != "" {
			decoded, _ := base64.StdEncoding.DecodeString(ev.Event.Data.Stderr)
			result.Stderr += string(decoded)
		}
		if ev.Event.End.ExitCode != 0 || lineContainsEnd(line) {
			// The end event may have exitCode 0, which is falsy in JSON.
			// We detect the end event by checking if the "end" field is present.
			result.ExitCode = ev.Event.End.ExitCode
			if ev.Event.End.Error != "" {
				result.Stderr += ev.Event.End.Error
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("e2b execute: read stream: %w", err)
	}
	return result, nil
}

// lineContainsEnd checks if a JSON line contains the end event.
// This is needed because exitCode 0 is the zero value in Go.
func lineContainsEnd(line string) bool {
	return strings.Contains(line, `"end"`) || strings.Contains(line, `'end'`)
}

// ----------------------------------------------------------------------
// Lifecycle
// ----------------------------------------------------------------------

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
