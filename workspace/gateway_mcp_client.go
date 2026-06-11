package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// GatewayClient is a host-side HTTP client for an in-workspace MCPGateway.
type GatewayClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGatewayClient creates a client for the MCP gateway HTTP API.
func NewGatewayClient(baseURL, token string) *GatewayClient {
	return &GatewayClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// WithHTTPClient overrides the default HTTP client.
func (c *GatewayClient) WithHTTPClient(client *http.Client) *GatewayClient {
	c.httpClient = client
	return c
}

// Health checks gateway liveness.
func (c *GatewayClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway health: %s: %s", resp.Status, string(b))
	}
	return nil
}

// ListMCPs returns full MCPClient specs (PyV2 MCPClient.model_dump compatible).
func (c *GatewayClient) ListMCPs(ctx context.Context) ([]MCPServerSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/mcps", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateway list mcps: %s: %s", resp.Status, string(b))
	}
	var list []MCPServerSpec
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// MakeMCPClient returns a GatewayMCPClient for the named server.
func (c *GatewayClient) MakeMCPClient(name string) *GatewayMCPClient {
	return &GatewayMCPClient{
		name:    name,
		gateway: c,
	}
}

// MakeMCPClientFromSpec wires a client from a GET /mcps spec entry.
func (c *GatewayClient) MakeMCPClientFromSpec(spec MCPServerSpec) *GatewayMCPClient {
	return &GatewayMCPClient{
		name:    spec.Name,
		spec:    spec,
		gateway: c,
	}
}

// RegisterMCP registers an upstream MCP on the gateway (POST /mcps).
func (c *GatewayClient) RegisterMCP(ctx context.Context, spec MCPServerSpec) error {
	var out map[string]any
	return c.doJSON(ctx, http.MethodPost, "/mcps", spec, &out)
}

// RemoveMCP unregisters an MCP from the gateway (DELETE /mcps/{name}).
func (c *GatewayClient) RemoveMCP(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/mcps/"+name, nil, nil)
}

func (c *GatewayClient) setAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *GatewayClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway %s %s: %s", method, path, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// GatewayMCPClient implements mcp.Client by forwarding to MCPGateway HTTP endpoints.
type GatewayMCPClient struct {
	name    string
	spec    MCPServerSpec
	gateway *GatewayClient
}

// Connect registers the upstream MCP on the gateway when spec is present.
// For clients created from GET /mcps (already registered), this is a no-op.
func (c *GatewayMCPClient) Connect(ctx context.Context, _ mcp.MCPConfig) error {
	if c == nil || c.gateway == nil || c.spec.MCPConfig.Type == "" {
		return nil
	}
	return c.gateway.RegisterMCP(ctx, c.spec)
}

// ListTools fetches tool schemas from GET /mcps/{name}/tools.
func (c *GatewayMCPClient) ListTools(ctx context.Context) ([]mcp.ToolInfo, error) {
	var tools []mcp.ToolInfo
	err := c.gateway.doJSON(ctx, http.MethodGet, "/mcps/"+c.name+"/tools", nil, &tools)
	return tools, err
}

// CallTool invokes POST /mcps/{name}/tools/{tool}.
func (c *GatewayMCPClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	var result any
	err := c.gateway.doJSON(ctx, http.MethodPost, "/mcps/"+c.name+"/tools/"+name, map[string]any{
		"arguments": args,
	}, &result)
	return result, err
}

// Close removes the MCP server from the gateway.
func (c *GatewayMCPClient) Close() error {
	return c.gateway.doJSON(context.Background(), http.MethodDelete, "/mcps/"+c.name, nil, nil)
}

// GatewayMCPTool is a tool.Tool that forwards calls through the MCP gateway.
type GatewayMCPTool struct {
	mcpName string
	info    mcp.ToolInfo
	gateway *GatewayClient
}

// NewGatewayMCPTool wraps a gateway-backed MCP tool for the host-side toolkit.
func NewGatewayMCPTool(mcpName string, info mcp.ToolInfo, gateway *GatewayClient) tool.Tool {
	return &GatewayMCPTool{mcpName: mcpName, info: info, gateway: gateway}
}

func (t *GatewayMCPTool) Name() string {
	return mcp.FormatToolName(t.mcpName, t.info.Name)
}

func (t *GatewayMCPTool) Description() string { return t.info.Description }

func (t *GatewayMCPTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.info.Description,
		Parameters:  t.info.Parameters,
	}
}

func (t *GatewayMCPTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	var result any
	err := t.gateway.doJSON(ctx, http.MethodPost,
		"/mcps/"+t.mcpName+"/tools/"+t.info.Name,
		map[string]any{"arguments": input},
		&result,
	)
	if err != nil {
		return nil, err
	}
	return tool.NewTextResponse(result), nil
}

func (t *GatewayMCPTool) IsMCPTool() bool  { return true }
func (t *GatewayMCPTool) MCPName() string  { return t.mcpName }
func (t *GatewayMCPTool) IsReadOnly() bool { return t.info.ReadOnly }

func (t *GatewayMCPTool) CheckPermissions(_ map[string]any, _ any) (tool.PermissionDecision, string, string, bool) {
	if t.info.ReadOnly {
		return tool.PermAllow, "This is a read-only MCP tool. Allowing execution.", "read-only mcp tool", false
	}
	return tool.PermAsk, "MCP tools must be explicitly allowed by the user.", "mcp tool default", false
}

// GatewayTools lists all tools exposed by registered MCP servers on the gateway.
func GatewayTools(ctx context.Context, gateway *GatewayClient) ([]tool.Tool, error) {
	servers, err := gateway.ListMCPs(ctx)
	if err != nil {
		return nil, err
	}
	var out []tool.Tool
	for _, s := range servers {
		client := gateway.MakeMCPClientFromSpec(s)
		tools, err := client.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("list tools %s: %w", s.Name, err)
		}
		for _, info := range tools {
			out = append(out, NewGatewayMCPTool(s.Name, info, gateway))
		}
	}
	return out, nil
}
