package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// MaxBodyBytes limits the response body size for safety.
const MaxBodyBytes = 2 * 1024 * 1024

// FetchTool lets agents retrieve content from a URL (HTTP GET).
type FetchTool struct {
	client  *http.Client
	timeout time.Duration
}

// NewFetchTool creates a FetchTool with a configurable timeout.
func NewFetchTool(timeout time.Duration) *FetchTool {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &FetchTool{
		client: &http.Client{Timeout: timeout},
	}
}

func (t *FetchTool) Name() string { return "web_fetch" }

func (t *FetchTool) Description() string {
	return "Fetch content from a URL via HTTP GET. Returns the response body as text. " +
		"Use this to read documentation, API responses, or any publicly accessible web page."
}

func (t *FetchTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to fetch content from.",
				},
				"max_length": map[string]any{
					"type":        "integer",
					"description": "Maximum number of characters to return (default: all up to 2MB limit).",
				},
			},
			"required": []string{"url"},
		},
	}
}

func (t *FetchTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return tool.NewTextResponse("WebFetchError: url is required"), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return tool.NewTextResponse("WebFetchError: url must start with http:// or https://"), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return tool.NewTextResponse(fmt.Sprintf("WebFetchError: invalid url: %v", err)), nil
	}
	req.Header.Set("User-Agent", "AgentScope/2.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return tool.NewTextResponse(fmt.Sprintf("WebFetchError: %v", err)), nil
	}
	defer resp.Body.Close()

	limit := MaxBodyBytes
	reader := io.LimitReader(resp.Body, int64(limit))
	body, err := io.ReadAll(reader)
	if err != nil {
		return tool.NewTextResponse(fmt.Sprintf("WebFetchError: read body: %v", err)), nil
	}

	text := string(body)
	if maxLen, ok := input["max_length"].(float64); ok && maxLen > 0 {
		if int(maxLen) < len(text) {
			text = text[:int(maxLen)]
		}
	}

	header := fmt.Sprintf("HTTP %d %s\nContent-Type: %s\nContent-Length: %d bytes\n\n",
		resp.StatusCode, resp.Status, resp.Header.Get("Content-Type"), len(body))

	return tool.NewTextResponse(header + text), nil
}

func (t *FetchTool) IsReadOnly() bool { return true }

func (t *FetchTool) CheckPermissions(_ map[string]any, _ any) (tool.PermissionDecision, string, string, bool) {
	return tool.PermAllow, "Web fetch is read-only.", "web_fetch", false
}

func (t *FetchTool) MatchRule(pattern string, _ map[string]any) bool {
	return pattern == ""
}

func (t *FetchTool) GenerateSuggestions(_ map[string]any) []tool.SuggestedRule {
	return []tool.SuggestedRule{{
		Name:     "suggested-tool-name",
		ToolName: t.Name(),
		Target:   "tool_name",
		Pattern:  t.Name(),
		Decision: tool.PermAllow,
	}}
}

var _ tool.Tool = (*FetchTool)(nil)
