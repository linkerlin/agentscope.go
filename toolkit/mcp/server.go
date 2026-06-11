package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// fullSession implements ClientSession, SessionWithTools and SessionWithResources
// so that per-session tools/resources can be registered via the MCP server.
type fullSession struct {
	sessionID        string
	ch               chan mcp.JSONRPCNotification
	init             bool
	sessionTools     map[string]mcpserver.ServerTool
	sessionResources map[string]mcpserver.ServerResource
	mu               sync.RWMutex
}

func newFullSession(id string) *fullSession {
	return &fullSession{
		sessionID:        id,
		ch:               make(chan mcp.JSONRPCNotification, 16),
		sessionTools:     make(map[string]mcpserver.ServerTool),
		sessionResources: make(map[string]mcpserver.ServerResource),
	}
}

func (s *fullSession) Initialize()                                         { s.init = true }
func (s *fullSession) Initialized() bool                                   { return s.init }
func (s *fullSession) NotificationChannel() chan<- mcp.JSONRPCNotification { return s.ch }
func (s *fullSession) SessionID() string                                   { return s.sessionID }

// GetSessionTools returns a copy of the session-specific tools.
func (s *fullSession) GetSessionTools() map[string]mcpserver.ServerTool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]mcpserver.ServerTool, len(s.sessionTools))
	for k, v := range s.sessionTools {
		copyMap[k] = v
	}
	return copyMap
}

// SetSessionTools replaces the session-specific tools.
func (s *fullSession) SetSessionTools(tools map[string]mcpserver.ServerTool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionTools = make(map[string]mcpserver.ServerTool, len(tools))
	for k, v := range tools {
		s.sessionTools[k] = v
	}
}

// GetSessionResources returns a copy of the session-specific resources.
func (s *fullSession) GetSessionResources() map[string]mcpserver.ServerResource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]mcpserver.ServerResource, len(s.sessionResources))
	for k, v := range s.sessionResources {
		copyMap[k] = v
	}
	return copyMap
}

// SetSessionResources replaces the session-specific resources.
func (s *fullSession) SetSessionResources(resources map[string]mcpserver.ServerResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionResources = make(map[string]mcpserver.ServerResource, len(resources))
	for k, v := range resources {
		s.sessionResources[k] = v
	}
}

// ServerAdapter exposes an AgentScope Agent as an MCP server.
// It supports tools, resources, prompts, and session-scoped capabilities.
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
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(true, true),
		mcpserver.WithPromptCapabilities(true),
	)

	// Register a default "chat" tool that proxies to the agent's Call method.
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

	// Register a default "chat_stream" tool that proxies to the agent's CallStream method.
	chatStreamTool := mcp.NewTool("chat_stream",
		mcp.WithDescription("Send a message to the agent and receive a streaming response."),
		mcp.WithString("message", mcp.Required(), mcp.Description("The message to send to the agent")),
	)
	mcpServer.AddTool(chatStreamTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

// RegisterResource adds a static resource to the MCP server.
// The handler is called when a client requests the resource by URI.
func (s *ServerAdapter) RegisterResource(uri, name, description, mimeType string, handler mcpserver.ResourceHandlerFunc) error {
	if s.server == nil {
		return fmt.Errorf("mcp server adapter: not started")
	}
	res := mcp.Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MIMEType:    mimeType,
	}
	s.server.AddResource(res, handler)
	return nil
}

// RegisterPrompt adds a prompt template to the MCP server.
// The handler receives the prompt arguments and returns the rendered prompt.
func (s *ServerAdapter) RegisterPrompt(name, description string, args []mcp.PromptArgument, handler mcpserver.PromptHandlerFunc) error {
	if s.server == nil {
		return fmt.Errorf("mcp server adapter: not started")
	}
	prompt := mcp.Prompt{
		Name:        name,
		Description: description,
		Arguments:   args,
	}
	s.server.AddPrompt(prompt, handler)
	return nil
}

// RegisterSessionTool adds a session-scoped tool to the MCP server.
// The tool is only available for the given session ID.
func (s *ServerAdapter) RegisterSessionTool(sessionID string, tool mcp.Tool, handler mcpserver.ToolHandlerFunc) error {
	if s.server == nil {
		return fmt.Errorf("mcp server adapter: not started")
	}
	return s.server.AddSessionTool(sessionID, tool, handler)
}

// RegisterSessionResource adds a session-scoped resource to the MCP server.
// The resource is only available for the given session ID.
func (s *ServerAdapter) RegisterSessionResource(sessionID string, uri, name, description, mimeType string, handler mcpserver.ResourceHandlerFunc) error {
	if s.server == nil {
		return fmt.Errorf("mcp server adapter: not started")
	}
	res := mcp.Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MIMEType:    mimeType,
	}
	return s.server.AddSessionResource(sessionID, res, handler)
}

// RegisterSessionPrompt adds a prompt to the MCP server.
// Note: mcp-go does not support session-scoped prompts; the prompt is added globally.
func (s *ServerAdapter) RegisterSessionPrompt(sessionID string, name, description string, args []mcp.PromptArgument, handler mcpserver.PromptHandlerFunc) error {
	if s.server == nil {
		return fmt.Errorf("mcp server adapter: not started")
	}
	_ = sessionID // reserved for future session-scoped prompt support
	prompt := mcp.Prompt{
		Name:        name,
		Description: description,
		Arguments:   args,
	}
	s.server.AddPrompt(prompt, handler)
	return nil
}

// RegisterSession registers a new client session with the MCP server.
// This is required before adding session-scoped tools or resources.
func (s *ServerAdapter) RegisterSession(ctx context.Context, sessionID string) error {
	if s.server == nil {
		return fmt.Errorf("mcp server adapter: not started")
	}
	return s.server.RegisterSession(ctx, newFullSession(sessionID))
}
