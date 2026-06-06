// Package observability provides integrations with external observability platforms.
package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LangSmithClient sends agent runs to the LangSmith tracing API.
type LangSmithClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	project    string
}

// NewLangSmithClient creates a LangSmith client.
func NewLangSmithClient(apiKey string) *LangSmithClient {
	return &LangSmithClient{
		apiKey:     apiKey,
		baseURL:    "https://api.smith.langchain.com",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithBaseURL sets a custom LangSmith endpoint (e.g., EU region or on-prem).
func (c *LangSmithClient) WithBaseURL(url string) *LangSmithClient {
	c.baseURL = url
	return c
}

// WithProject sets the project name for all runs.
func (c *LangSmithClient) WithProject(name string) *LangSmithClient {
	c.project = name
	return c
}

// WithHTTPClient sets a custom HTTP client.
func (c *LangSmithClient) WithHTTPClient(client *http.Client) *LangSmithClient {
	c.httpClient = client
	return c
}

// Run represents a single traced execution unit in LangSmith.
type Run struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	RunType      string         `json:"run_type"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      *time.Time     `json:"end_time,omitempty"`
	Inputs       map[string]any `json:"inputs,omitempty"`
	Outputs      map[string]any `json:"outputs,omitempty"`
	Error        *string        `json:"error,omitempty"`
	ParentRunID  *string        `json:"parent_run_id,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	SessionName  string         `json:"session_name,omitempty"`
}

// CreateRunsBatch uploads a batch of runs to LangSmith.
func (c *LangSmithClient) CreateRunsBatch(ctx context.Context, runs []Run) error {
	body := map[string]any{"post": runs}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("langsmith: marshal runs: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/runs/batch", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("langsmith: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("langsmith: %s: %s", resp.Status, string(b))
	}
	return nil
}

// CreateRun uploads a single run.
func (c *LangSmithClient) CreateRun(ctx context.Context, run Run) error {
	return c.CreateRunsBatch(ctx, []Run{run})
}
