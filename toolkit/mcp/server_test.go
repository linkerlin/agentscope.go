package mcp

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/mark3labs/mcp-go/mcp"
)

type mockAgent struct{}

func (m *mockAgent) Name() string { return "mock" }
func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(), nil
}
func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build()
	close(ch)
	return ch, nil
}

var _ agent.Agent = (*mockAgent)(nil)

func TestServerAdapter_Start(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	server, err := adapt.Start(context.Background())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if adapt.Server() == nil {
		t.Fatal("expected Server() to return non-nil")
	}
	// Start again should return the same server
	server2, err := adapt.Start(context.Background())
	if err != nil {
		t.Fatalf("expected success on second start, got %v", err)
	}
	if server2 != server {
		t.Fatal("expected same server instance")
	}
}

func TestServerAdapter_RegisterResource(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	_, _ = adapt.Start(context.Background())

	err := adapt.RegisterResource("memory://agent/state", "agent-state", "Current agent state", "application/json",
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{URI: "memory://agent/state", MIMEType: "application/json", Text: `{"status":"ok"}`},
			}, nil
		})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestServerAdapter_RegisterResource_NotStarted(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	err := adapt.RegisterResource("memory://test", "test", "", "", nil)
	if err == nil {
		t.Fatal("expected error when not started")
	}
}

func TestServerAdapter_RegisterPrompt(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	_, _ = adapt.Start(context.Background())

	err := adapt.RegisterPrompt("greeting", "A greeting prompt", nil,
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: "greeting",
				Messages: []mcp.PromptMessage{
					{Role: mcp.RoleUser, Content: mcp.TextContent{Type: "text", Text: "Hello!"}},
				},
			}, nil
		})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestServerAdapter_RegisterPrompt_NotStarted(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	err := adapt.RegisterPrompt("test", "", nil, nil)
	if err == nil {
		t.Fatal("expected error when not started")
	}
}

func TestServerAdapter_RegisterSessionTool(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	_, _ = adapt.Start(context.Background())

	if err := adapt.RegisterSession(context.Background(), "sess-1"); err != nil {
		t.Fatalf("register session: %v", err)
	}

	tool := mcp.NewTool("session_tool", mcp.WithDescription("A session-scoped tool"))
	err := adapt.RegisterSessionTool("sess-1", tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestServerAdapter_RegisterSessionResource(t *testing.T) {
	adapt := NewServerAdapter(&mockAgent{}, "test-agent", "1.0.0")
	_, _ = adapt.Start(context.Background())

	if err := adapt.RegisterSession(context.Background(), "sess-1"); err != nil {
		t.Fatalf("register session: %v", err)
	}

	err := adapt.RegisterSessionResource("sess-1", "memory://sess/state", "sess-state", "Session state", "application/json",
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{URI: "memory://sess/state", MIMEType: "application/json", Text: `{}`},
			}, nil
		})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}
