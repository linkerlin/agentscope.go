package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

type stubMCPClient struct {
	tools []mcp.ToolInfo
}

func (s *stubMCPClient) Connect(ctx context.Context, cfg mcp.MCPConfig) error { return nil }
func (s *stubMCPClient) ListTools(ctx context.Context) ([]mcp.ToolInfo, error) {
	return s.tools, nil
}
func (s *stubMCPClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return map[string]any{"tool": name, "args": args}, nil
}
func (s *stubMCPClient) Close() error { return nil }

func TestMCPGateway_ListAndCallTool(t *testing.T) {
	gw := NewMCPGateway("secret")
	gw.RegisterServer("demo", &stubMCPClient{
		tools: []mcp.ToolInfo{{Name: "echo", Description: "echo", Parameters: map[string]any{}}},
	})
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/mcps", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["name"] != "demo" {
		t.Fatalf("unexpected list: %#v", list)
	}

	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/mcps/demo/tools", nil)
	req2.Header.Set("Authorization", "Bearer secret")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var tools []mcp.ToolInfo
	if err := json.NewDecoder(resp2.Body).Decode(&tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestGatewayMCPClient_Tools(t *testing.T) {
	gw := NewMCPGateway("")
	gw.RegisterServer("s1", &stubMCPClient{
		tools: []mcp.ToolInfo{{Name: "add", Description: "add two numbers", Parameters: map[string]any{}}},
	})
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	client := NewGatewayClient(srv.URL, "")
	tools, err := GatewayTools(context.Background(), client)
	if err != nil || len(tools) != 1 {
		t.Fatalf("GatewayTools: err=%v len=%d", err, len(tools))
	}
	if tools[0].Name() != "mcp__s1__add" {
		t.Fatalf("unexpected tool name: %s", tools[0].Name())
	}
	resp, err := tools[0].Execute(context.Background(), map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() == "" {
		t.Fatal("empty response")
	}
}

func TestE2BMCPGatewaySidecar(t *testing.T) {
	ctx := context.Background()
	sidecar, err := StartE2BMCPGateway(ctx, E2BMCPConfig{}, func(g *MCPGateway) {
		g.RegisterServer("x", &stubMCPClient{
			tools: []mcp.ToolInfo{{Name: "ping", Description: "ping"}},
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sidecar.Close(ctx)

	if err := sidecar.NewGatewayClient("").Health(ctx); err != nil {
		t.Fatal(err)
	}
}
