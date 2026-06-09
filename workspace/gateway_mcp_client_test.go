package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

func TestGatewayClient_RemoveMCP(t *testing.T) {
	gw := NewMCPGateway("")
	gw.RegisterServer("demo", &stubMCPForClient{
		tools: []mcp.ToolInfo{{Name: "echo"}},
	})
	gatewaySrv := httptest.NewServer(gw.Handler())
	defer gatewaySrv.Close()

	client := NewGatewayClient(gatewaySrv.URL, "")
	list, err := client.ListMCPs(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "demo" {
		t.Fatalf("ListMCPs: err=%v list=%#v", err, list)
	}

	if err := client.RemoveMCP(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	list, err = client.ListMCPs(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty after remove, got %#v err=%v", list, err)
	}
}

type stubMCPForClient struct {
	tools []mcp.ToolInfo
}

func (s *stubMCPForClient) Connect(context.Context, mcp.MCPConfig) error { return nil }
func (s *stubMCPForClient) ListTools(context.Context) ([]mcp.ToolInfo, error) {
	return s.tools, nil
}
func (s *stubMCPForClient) CallTool(context.Context, string, map[string]any) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (s *stubMCPForClient) Close() error { return nil }

func TestGatewayMCPClient_ConnectPostsSpec(t *testing.T) {
	var posted MCPServerSpec
	gw := NewMCPGateway("")
	gw.Handler() // ensure handler built
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/mcps", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&posted)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	spec := MCPServerSpec{
		Name:       "x",
		IsStateful: true,
		MCPConfig:  MCPConfigSpec{Type: "stdio_mcp", Command: "echo"},
	}
	client := NewGatewayClient(srv.URL, "")
	gwClient := client.MakeMCPClientFromSpec(spec)
	if err := gwClient.Connect(context.Background(), mcp.MCPConfig{}); err != nil {
		t.Fatal(err)
	}
	if posted.Name != "x" || posted.MCPConfig.Command != "echo" {
		t.Fatalf("unexpected posted spec: %#v", posted)
	}
}
