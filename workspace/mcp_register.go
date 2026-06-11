package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// MCPRegisterRequest describes an MCP server registration for POST /mcps.
// Aligned with PyV2 MCPClient.model_dump() transport fields.
type MCPRegisterRequest struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // stdio | sse | http
	Endpoint  string            `json:"endpoint,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// MCPGatewayConfig bootstraps a gateway with token and preconfigured servers.
type MCPGatewayConfig struct {
	Token   string               `json:"token,omitempty"`
	Servers []MCPRegisterRequest `json:"servers,omitempty"`
}

// RegisterClientFromRequest connects an upstream MCP client and registers it on the gateway.
func (g *MCPGateway) RegisterClientFromRequest(ctx context.Context, req MCPRegisterRequest, spec MCPServerSpec) error {
	if g == nil {
		return fmt.Errorf("mcp gateway: nil gateway")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("mcp gateway: name required")
	}
	g.mu.RLock()
	_, exists := g.servers[req.Name]
	g.mu.RUnlock()
	if exists {
		return fmt.Errorf("mcp gateway: %q already exists", req.Name)
	}
	client, err := BuildMCPClient(req)
	if err != nil {
		return err
	}
	if err := client.Connect(ctx, mcp.MCPConfig{Name: req.Name, Endpoint: req.Endpoint}); err != nil {
		_ = client.Close()
		return fmt.Errorf("mcp gateway: connect %s: %w", req.Name, err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.servers[req.Name]; exists {
		_ = client.Close()
		return fmt.Errorf("mcp gateway: %q already exists", req.Name)
	}
	if spec.Name == "" {
		spec = SpecFromRegisterRequest(req)
	}
	if !spec.IsStateful {
		spec.IsStateful = true
	}
	g.servers[req.Name] = mcpServerEntry{client: client, spec: spec}
	return nil
}

// LoadConfig registers all servers from a gateway config document.
func (g *MCPGateway) LoadConfig(ctx context.Context, cfg MCPGatewayConfig) error {
	for _, req := range cfg.Servers {
		if err := g.RegisterClientFromRequest(ctx, req, SpecFromRegisterRequest(req)); err != nil {
			return err
		}
	}
	return nil
}

// BuildMCPClient constructs an MCP client from a registration request.
func BuildMCPClient(req MCPRegisterRequest) (mcp.Client, error) {
	name := req.Name
	if name == "" {
		name = "mcp"
	}
	b := mcp.NewClientBuilder(name)
	switch strings.ToLower(req.Transport) {
	case "", "stdio":
		if req.Command == "" {
			return nil, fmt.Errorf("mcp gateway: command required for stdio transport")
		}
		if len(req.Env) > 0 {
			b.StdioTransportWithEnv(req.Command, req.Env, req.Args...)
		} else {
			b.StdioTransport(req.Command, req.Args...)
		}
	case "sse":
		if req.Endpoint == "" {
			return nil, fmt.Errorf("mcp gateway: endpoint required for sse transport")
		}
		b.SSETransport(req.Endpoint)
	case "http":
		if req.Endpoint == "" {
			return nil, fmt.Errorf("mcp gateway: endpoint required for http transport")
		}
		b.StreamableHTTPTransport(req.Endpoint)
	default:
		return nil, fmt.Errorf("mcp gateway: unknown transport %q", req.Transport)
	}
	for k, v := range req.Headers {
		b.Header(k, v)
	}
	return b.Build()
}
