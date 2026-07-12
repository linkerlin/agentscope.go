package observability

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// LangfuseClient sends trace data to the Langfuse ingestion API
// (https://cloud.langfuse.com or self-hosted). Langfuse is a popular
// open-source LLM observability platform — this is the Langfuse counterpart to
// LangSmithClient. Events are batched and POSTed to /api/public/ingestion with
// Basic auth (publicKey:secretKey).
type LangfuseClient struct {
	publicKey  string
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

// NewLangfuseClient creates a client. publicKey/secretKey come from your
// Langfuse project settings.
func NewLangfuseClient(publicKey, secretKey string) *LangfuseClient {
	return &LangfuseClient{
		publicKey:  publicKey,
		secretKey:  secretKey,
		baseURL:    "https://cloud.langfuse.com",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithBaseURL sets a custom Langfuse endpoint (self-hosted / on-prem).
func (c *LangfuseClient) WithBaseURL(url string) *LangfuseClient {
	c.baseURL = url
	return c
}

// WithHTTPClient sets a custom HTTP client.
func (c *LangfuseClient) WithHTTPClient(client *http.Client) *LangfuseClient {
	c.httpClient = client
	return c
}

// LangfuseEvent is one entry in a Langfuse ingestion batch. Langfuse merges
// events by their body id, so sending the same trace/generation id twice with
// different fields updates the record.
type LangfuseEvent struct {
	ID        string         `json:"id"`        // unique event id (uuid)
	Type      string         `json:"type"`      // trace-create | span-create | generation-create
	Timestamp string         `json:"timestamp"` // RFC3339
	Body      map[string]any `json:"body"`
}

// Event type constants.
const (
	LangfuseTraceCreate      = "trace-create"
	LangfuseSpanCreate       = "span-create"
	LangfuseGenerationCreate = "generation-create"
)

// NewLangfuseEvent builds an event with a fresh uuid and RFC3339 timestamp.
func NewLangfuseEvent(eventType string, body map[string]any) LangfuseEvent {
	return LangfuseEvent{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Body:      body,
	}
}

// Ingest posts a batch of events to Langfuse.
func (c *LangfuseClient) Ingest(ctx context.Context, events []LangfuseEvent) error {
	if len(events) == 0 {
		return nil
	}
	if c.publicKey == "" || c.secretKey == "" {
		return fmt.Errorf("langfuse: public/secret key not set")
	}
	data, err := json.Marshal(map[string]any{"batch": events})
	if err != nil {
		return fmt.Errorf("langfuse: marshal batch: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/public/ingestion", bytes.NewReader(data))
	if err != nil {
		return err
	}
	cred := base64.StdEncoding.EncodeToString([]byte(c.publicKey + ":" + c.secretKey))
	req.Header.Set("Authorization", "Basic "+cred)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("langfuse: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("langfuse: %s: %s", resp.Status, string(b))
	}
	return nil
}

// IngestOne posts a single event (convenience).
func (c *LangfuseClient) IngestOne(ctx context.Context, ev LangfuseEvent) error {
	return c.Ingest(ctx, []LangfuseEvent{ev})
}
