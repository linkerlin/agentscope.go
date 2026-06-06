package mcp

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// ServerAdapter exposes an AgentScope Agent as an MCP server.
// This is a skeleton implementation; the full server lifecycle and
// resource/prompt handlers can be added incrementally.
type ServerAdapter struct {
	agent   agent.Agent
	name    string
	version string
	server  *mcpserver.MCPServer
}

// NewServerAdapter creates a new MCP server adapter for the given agent.
func NewServerAdapter(a agent.Agent, name, version string) *ServerAdapter {
	return &ServerAdapter{
		agent:   a,
		name:    name,
		version: version,
	}
}

// Start initialises the MCP server and registers the agent's capabilities as tools.
// It returns the underlying MCPServer for further configuration (transport, etc.).
func (s *ServerAdapter) Start(ctx context.Context) (*mcpserver.MCPServer, error) {
	if s.server != nil {
		return s.server, nil
	}

	mcpServer := mcpserver.NewMCPServer(
		s.name,
		s.version,
	)

	// Register a default "chat" tool that proxies to the agent's Call method.
	// Concrete implementations can extend this with additional tools.
	chatTool := mcp.NewTool("chat",
		mcp.WithDescription("Send a message to the agent and receive a response."),
		mcp.WithString("message", mcp.Required(), mcp.Description("The message to send to the agent")),
	)
	mcpServer.AddTool(chatTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		msg, err := request.RequireString("message")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		userMsg := message.NewMsg().Role(message.RoleUser).TextContent(msg).Build()
		resp, err := s.agent.Call(ctx, userMsg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(resp.GetTextContent()), nil
	})

	s.server = mcpServer
	return mcpServer, nil
}

// Server returns the underlying MCPServer (nil if Start has not been called).
func (s *ServerAdapter) Server() *mcpserver.MCPServer {
	return s.server
}
