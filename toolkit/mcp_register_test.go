package toolkit

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
	mcpclient "github.com/mark3labs/mcp-go/client"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func createTestMCPClient(t *testing.T) mcp.Client {
	s := mcpserver.NewMCPServer("test-server", "1.0.0")
	s.AddTool(gomcp.NewTool("echo", gomcp.WithDescription("echo")), func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		return gomcp.NewToolResultText("ok"), nil
	})
	s.AddTool(gomcp.NewTool("add", gomcp.WithDescription("add")), func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		return gomcp.NewToolResultText("2"), nil
	})
	raw, err := mcpclient.NewInProcessClient(s)
	if err != nil {
		t.Fatal(err)
	}
	c := mcp.NewSDKClient(raw)
	if err := c.Connect(context.Background(), mcp.MCPConfig{}); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestToolkit_RegisterMCPClient(t *testing.T) {
	tk := NewToolkit()
	mc := createTestMCPClient(t)
	defer mc.Close()

	ctx := context.Background()
	if err := tk.RegisterMCPClient(ctx, "local", mc); err != nil {
		t.Fatal(err)
	}

	tools := tk.Registry.List()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Name() != "local/echo" && tool.Name() != "local/add" {
			t.Fatalf("unexpected tool name: %s", tool.Name())
		}
	}
}

func TestToolkit_RegisterMCPClient_WithGroup(t *testing.T) {
	tk := NewToolkit()
	mc := createTestMCPClient(t)
	defer mc.Close()

	ctx := context.Background()
	if err := tk.RegisterMCPClient(ctx, "local", mc, WithMCPGroup("g1")); err != nil {
		t.Fatal(err)
	}

	if err := tk.Groups.SetGroupActive("g1", true); err != nil {
		t.Fatal(err)
	}
	active := tk.ActiveTools()
	if len(active) != 2 {
		t.Fatalf("expected 2 active tools, got %d", len(active))
	}
}

func TestToolkit_RegisterMCPClient_EnableTools(t *testing.T) {
	tk := NewToolkit()
	mc := createTestMCPClient(t)
	defer mc.Close()

	ctx := context.Background()
	if err := tk.RegisterMCPClient(ctx, "local", mc, WithMCPEnableTools("echo")); err != nil {
		t.Fatal(err)
	}

	tools := tk.Registry.List()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name() != "local/echo" {
		t.Fatalf("unexpected tool: %s", tools[0].Name())
	}
}

func TestToolkit_RegisterMCPClient_DisableTools(t *testing.T) {
	tk := NewToolkit()
	mc := createTestMCPClient(t)
	defer mc.Close()

	ctx := context.Background()
	if err := tk.RegisterMCPClient(ctx, "local", mc, WithMCPDisableTools("add")); err != nil {
		t.Fatal(err)
	}

	tools := tk.Registry.List()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name() != "local/echo" {
		t.Fatalf("unexpected tool: %s", tools[0].Name())
	}
}

func TestToolkit_RegisterMCPClient_Execute(t *testing.T) {
	tk := NewToolkit()
	mc := createTestMCPClient(t)
	defer mc.Close()

	ctx := context.Background()
	if err := tk.RegisterMCPClient(ctx, "local", mc); err != nil {
		t.Fatal(err)
	}

	resp, err := tk.ExecuteTool(ctx, "local/echo", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected result: %s", resp.GetTextContent())
	}
}

func TestShouldRegisterMCP(t *testing.T) {
	if !shouldRegisterMCP("a", nil, nil) {
		t.Fatal("expected true")
	}
	if shouldRegisterMCP("a", nil, []string{"a"}) {
		t.Fatal("expected false when disabled")
	}
	if shouldRegisterMCP("a", []string{"b"}, nil) {
		t.Fatal("expected false when not in enable list")
	}
	if !shouldRegisterMCP("a", []string{"a", "b"}, []string{"a"}) {
		t.Fatal("enable list should take precedence")
	}
}
