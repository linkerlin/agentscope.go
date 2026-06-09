package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/gateway"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
	"github.com/linkerlin/agentscope.go/workspace"
)

type sidecarStubMCP struct{}

func (s *sidecarStubMCP) Connect(context.Context, mcp.MCPConfig) error { return nil }
func (s *sidecarStubMCP) ListTools(context.Context) ([]mcp.ToolInfo, error) {
	return []mcp.ToolInfo{{Name: "ping", Description: "ping"}}, nil
}
func (s *sidecarStubMCP) CallTool(context.Context, string, map[string]any) (any, error) {
	return "pong", nil
}
func (s *sidecarStubMCP) Close() error { return nil }

func TestE2E_MCPSidecar_PersistsAndRestores(t *testing.T) {
	t.Setenv("MCP_GATEWAY_AUTO_START", "false")

	ctx := context.Background()
	token := "test-token"
	sidecar, err := workspace.StartMCPGatewaySidecar(ctx, workspace.MCPGatewaySidecarConfig{Token: token}, func(gw *workspace.MCPGateway) {
		gw.RegisterServer("demo", &sidecarStubMCP{})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sidecar.Close(ctx)

	root := t.TempDir()
	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(ctx, &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := gateway.NewWorkspaceManager(root, "").
		WithDefaultMCPGateway(sidecar.HostURL, token)

	if err := wsMgr.AddMCP(ctx, storage, "u1", "a1", "s1", gateway.MCPRegistration{
		Name: "demo",
	}); err != nil {
		t.Fatal(err)
	}

	mcpFile := filepath.Join(root, "u1", "a1", "s1", ".mcp")
	if _, err := os.Stat(mcpFile); err != nil {
		t.Fatalf(".mcp not written: %v", err)
	}

	wsMgr2 := gateway.NewWorkspaceManager(root, "").
		WithDefaultMCPGateway(sidecar.HostURL, token)
	list, err := wsMgr2.ListMCPs(ctx, storage, "u1", "a1", "s1")
	if err != nil || len(list) != 1 || list[0].Name != "demo" {
		t.Fatalf("restore: err=%v list=%#v", err, list)
	}
	if !list[0].IsHealthy {
		t.Fatal("expected healthy restored mcp")
	}
}

func TestE2E_MCPSidecar_HTTPAttach(t *testing.T) {
	ctx := context.Background()
	token := "secret"
	sidecar, err := workspace.StartMCPGatewaySidecar(ctx, workspace.MCPGatewaySidecarConfig{Token: token}, func(gw *workspace.MCPGateway) {
		gw.RegisterServer("demo", &sidecarStubMCP{})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sidecar.Close(ctx)

	root := t.TempDir()
	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(ctx, &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := gateway.NewWorkspaceManager(root, "").
		WithDefaultMCPGateway(sidecar.HostURL, token)
	srv := gateway.NewServer(&mockV2Agent{})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	body, _ := json.Marshal(map[string]any{"name": "demo"})
	req := httptest.NewRequest(http.MethodPost, "/workspace/mcp?agent_id=a1&session_id=s1", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, "u1"))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("attach via http: %d %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(root, "u1", "a1", "s1", ".mcp")); err != nil {
		t.Fatalf(".mcp missing after http add: %v", err)
	}
}
