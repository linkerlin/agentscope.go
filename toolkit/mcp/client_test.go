package mcp

import (
	"context"
	"strings"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func createTestMCPServer() *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("test-server", "1.0.0")
	s.AddTool(mcp.NewTool("echo", mcp.WithDescription("echo tool")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var msg string
		if args, ok := req.Params.Arguments.(map[string]any); ok {
			if v, ok := args["msg"].(string); ok {
				msg = v
			}
		}
		return mcp.NewToolResultText("echo: " + msg), nil
	})
	s.AddTool(mcp.NewTool("add", mcp.WithDescription("add tool")), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("added"), nil
	})
	return s
}

func TestSDKClient_InProcess(t *testing.T) {
	s := createTestMCPServer()
	raw, err := mcpclient.NewInProcessClient(s)
	if err != nil {
		t.Fatal(err)
	}
	c := NewSDKClient(raw)
	ctx := context.Background()
	if err := c.Connect(ctx, MCPConfig{}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	found := false
	for _, ti := range tools {
		if ti.Name == "echo" {
			found = true
			if ti.Description != "echo tool" {
				t.Fatalf("unexpected description: %s", ti.Description)
			}
			if ti.Parameters["type"] != "object" {
				t.Fatalf("unexpected schema type: %v", ti.Parameters["type"])
			}
		}
	}
	if !found {
		t.Fatal("expected echo tool")
	}

	res, err := c.CallTool(ctx, "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.(string), "echo: hello") {
		t.Fatalf("unexpected result: %v", res)
	}
}

func TestSDKClient_NotInitialized(t *testing.T) {
	s := createTestMCPServer()
	raw, _ := mcpclient.NewInProcessClient(s)
	c := NewSDKClient(raw)

	ctx := context.Background()
	_, err := c.ListTools(ctx)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected not initialized error, got %v", err)
	}
	_, err = c.CallTool(ctx, "echo", nil)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected not initialized error, got %v", err)
	}
}

func TestClientBuilder_TransportNotConfigured(t *testing.T) {
	b := NewClientBuilder("test")
	_, err := b.Build()
	if err == nil || !strings.Contains(err.Error(), "transport not configured") {
		t.Fatalf("expected transport not configured error, got %v", err)
	}
}

func TestClientBuilder_SSEWithQueryParams(t *testing.T) {
	b := NewClientBuilder("test").
		SSETransport("http://localhost:8080/sse").
		QueryParam("token", "abc").
		Header("Authorization", "Bearer x")
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = c
}

func TestClientBuilder_HTTPWithQueryParams(t *testing.T) {
	b := NewClientBuilder("test").
		StreamableHTTPTransport("http://localhost:8080/mcp").
		QueryParam("env", "prod")
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = c
}

func TestFormatCallToolResult(t *testing.T) {
	res := mcp.NewToolResultText("hello")
	out := formatCallToolResult(res)
	if out != "hello" {
		t.Fatalf("expected hello, got %v", out)
	}

	res2 := &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Text: "a"}, mcp.TextContent{Text: "b"}}, IsError: false}
	out2 := formatCallToolResult(res2)
	if !strings.Contains(out2.(string), "a") || !strings.Contains(out2.(string), "b") {
		t.Fatalf("expected combined text, got %v", out2)
	}

	res3 := mcp.NewToolResultError("oops")
	out3 := formatCallToolResult(res3)
	if out3 != "oops" {
		t.Fatalf("expected oops, got %v", out3)
	}
}
