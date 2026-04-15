package rag

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TikaClient uses a remote Apache Tika Server to extract text from documents.
// It requires a running Tika REST service (e.g. http://localhost:9998).
type TikaClient struct {
	BaseURL string
	Client  *http.Client
}

// NewTikaClient creates a client for the given Tika Server base URL.
func NewTikaClient(baseURL string) *TikaClient {
	if baseURL == "" {
		baseURL = "http://localhost:9998"
	}
	return &TikaClient{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ParseFile extracts text from a local file via the Tika Server.
func (c *TikaClient) ParseFile(ctx context.Context, path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tika: open file: %w", err)
	}
	defer f.Close()
	return c.Parse(ctx, f)
}

// Parse extracts text from an io.Reader via the Tika Server.
func (c *TikaClient) Parse(ctx context.Context, r io.Reader) (*Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+"/tika", r)
	if err != nil {
		return nil, fmt.Errorf("tika: create request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tika: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tika: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tika: read response: %w", err)
	}

	return &Document{
		Text:     string(data),
		Metadata: map[string]any{},
	}, nil
}
