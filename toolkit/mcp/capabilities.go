package mcp

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// --- Prompts ---

// PromptArgument represents an argument accepted by an MCP prompt.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptInfo is metadata for an MCP prompt.
type PromptInfo struct {
	Name        string
	Description string
	Arguments   []PromptArgument
}

// PromptMessage is a single message in a resolved prompt.
type PromptMessage struct {
	Role    string
	Content string
}

// PromptResult is the output of GetPrompt.
type PromptResult struct {
	Description string
	Messages    []PromptMessage
}

// PromptsClient is an optional capability for MCP clients that support prompts.
type PromptsClient interface {
	Client
	ListPrompts(ctx context.Context) ([]PromptInfo, error)
	GetPrompt(ctx context.Context, name string, args map[string]string) (*PromptResult, error)
}

// ListPrompts is a helper that calls ListPrompts if the client supports it.
func ListPrompts(ctx context.Context, c Client) ([]PromptInfo, error) {
	if pc, ok := c.(PromptsClient); ok {
		return pc.ListPrompts(ctx)
	}
	return nil, fmt.Errorf("mcp: client %T does not support prompts", c)
}

// GetPrompt is a helper that calls GetPrompt if the client supports it.
func GetPrompt(ctx context.Context, c Client, name string, args map[string]string) (*PromptResult, error) {
	if pc, ok := c.(PromptsClient); ok {
		return pc.GetPrompt(ctx, name, args)
	}
	return nil, fmt.Errorf("mcp: client %T does not support prompts", c)
}

// --- Resources ---

// ResourceInfo is metadata for an MCP resource.
type ResourceInfo struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

// ResourceContent holds the content read from a resource URI.
type ResourceContent struct {
	URI      string
	MimeType string
	Text     string
	Blob     []byte
}

// ResourcesClient is an optional capability for MCP clients that support resources.
type ResourcesClient interface {
	Client
	ListResources(ctx context.Context) ([]ResourceInfo, error)
	ReadResource(ctx context.Context, uri string) (*ResourceContent, error)
}

// ListResources is a helper that calls ListResources if the client supports it.
func ListResources(ctx context.Context, c Client) ([]ResourceInfo, error) {
	if rc, ok := c.(ResourcesClient); ok {
		return rc.ListResources(ctx)
	}
	return nil, fmt.Errorf("mcp: client %T does not support resources", c)
}

// ReadResource is a helper that calls ReadResource if the client supports it.
func ReadResource(ctx context.Context, c Client, uri string) (*ResourceContent, error) {
	if rc, ok := c.(ResourcesClient); ok {
		return rc.ReadResource(ctx, uri)
	}
	return nil, fmt.Errorf("mcp: client %T does not support resources", c)
}

// --- Sampling ---

// SamplingMessage is a message in a sampling request.
type SamplingMessage struct {
	Role    string
	Content string
}

// SamplingRequest represents a sampling/createMessage request.
type SamplingRequest struct {
	Messages      []SamplingMessage
	SystemPrompt  string
	MaxTokens     int
	Temperature   *float64
	StopSequences []string
}

// SamplingResponse represents the result of a sampling request.
type SamplingResponse struct {
	Role       string
	Content    string
	Model      string
	StopReason string
}

// SamplingClient is an optional capability for MCP clients that support sampling
// (server→client LLM proxy).
type SamplingClient interface {
	Client
	CreateMessage(ctx context.Context, req SamplingRequest) (*SamplingResponse, error)
}

// CreateMessage is a helper that calls CreateMessage if the client supports it.
func CreateMessage(ctx context.Context, c Client, req SamplingRequest) (*SamplingResponse, error) {
	if sc, ok := c.(SamplingClient); ok {
		return sc.CreateMessage(ctx, req)
	}
	return nil, fmt.Errorf("mcp: client %T does not support sampling", c)
}

// --- SDKClient Implementations ---

// ListPrompts implements PromptsClient.
func (c *SDKClient) ListPrompts(ctx context.Context) ([]PromptInfo, error) {
	if !c.initialized {
		return nil, fmt.Errorf("mcp: client not initialized")
	}
	res, err := c.inner.ListPrompts(ctx, mcp.ListPromptsRequest{})
	if err != nil {
		return nil, err
	}
	var infos []PromptInfo
	for _, p := range res.Prompts {
		pi := PromptInfo{
			Name:        p.Name,
			Description: p.Description,
		}
		for _, a := range p.Arguments {
			pi.Arguments = append(pi.Arguments, PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}
		infos = append(infos, pi)
	}
	return infos, nil
}

// GetPrompt implements PromptsClient.
func (c *SDKClient) GetPrompt(ctx context.Context, name string, args map[string]string) (*PromptResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("mcp: client not initialized")
	}
	res, err := c.inner.GetPrompt(ctx, mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, err
	}
	result := &PromptResult{
		Description: res.Description,
	}
	for _, msg := range res.Messages {
		result.Messages = append(result.Messages, convertPromptMessage(msg))
	}
	return result, nil
}

// ListResources implements ResourcesClient.
func (c *SDKClient) ListResources(ctx context.Context) ([]ResourceInfo, error) {
	if !c.initialized {
		return nil, fmt.Errorf("mcp: client not initialized")
	}
	res, err := c.inner.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return nil, err
	}
	var infos []ResourceInfo
	for _, r := range res.Resources {
		infos = append(infos, ResourceInfo{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MIMEType,
		})
	}
	return infos, nil
}

// ReadResource implements ResourcesClient.
func (c *SDKClient) ReadResource(ctx context.Context, uri string) (*ResourceContent, error) {
	if !c.initialized {
		return nil, fmt.Errorf("mcp: client not initialized")
	}
	res, err := c.inner.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: uri,
		},
	})
	if err != nil {
		return nil, err
	}
	if len(res.Contents) == 0 {
		return &ResourceContent{URI: uri}, nil
	}
	return convertResourceContent(res.Contents[0]), nil
}

// --- Conversion helpers ---

func convertPromptMessage(msg mcp.PromptMessage) PromptMessage {
	pm := PromptMessage{Role: string(msg.Role)}
	switch c := msg.Content.(type) {
	case mcp.TextContent:
		pm.Content = c.Text
	case *mcp.TextContent:
		pm.Content = c.Text
	default:
		pm.Content = fmt.Sprintf("%v", c)
	}
	return pm
}

func convertResourceContent(rc mcp.ResourceContents) *ResourceContent {
	out := &ResourceContent{}
	switch v := rc.(type) {
	case mcp.TextResourceContents:
		out.URI = v.URI
		out.MimeType = v.MIMEType
		out.Text = v.Text
	case *mcp.TextResourceContents:
		out.URI = v.URI
		out.MimeType = v.MIMEType
		out.Text = v.Text
	case mcp.BlobResourceContents:
		out.URI = v.URI
		out.MimeType = v.MIMEType
		if v.Blob != "" {
			out.Blob, _ = base64.StdEncoding.DecodeString(v.Blob)
		}
	case *mcp.BlobResourceContents:
		out.URI = v.URI
		out.MimeType = v.MIMEType
		if v.Blob != "" {
			out.Blob, _ = base64.StdEncoding.DecodeString(v.Blob)
		}
	default:
		out.Text = fmt.Sprintf("%v", rc)
	}
	return out
}
