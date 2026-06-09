package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/runcontext"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
	"github.com/linkerlin/agentscope.go/workspace"
)

type wsToolsStubMCP struct{}

func (s *wsToolsStubMCP) Connect(context.Context, mcp.MCPConfig) error { return nil }
func (s *wsToolsStubMCP) ListTools(context.Context) ([]mcp.ToolInfo, error) {
	return []mcp.ToolInfo{{Name: "ping", Description: "ping tool"}}, nil
}
func (s *wsToolsStubMCP) CallTool(context.Context, string, map[string]any) (any, error) {
	return "pong", nil
}
func (s *wsToolsStubMCP) Close() error { return nil }

func startStubGateway(t *testing.T, register func(*workspace.MCPGateway)) *httptest.Server {
	t.Helper()
	gw := workspace.NewMCPGateway("secret")
	if register != nil {
		register(gw)
	}
	return httptest.NewServer(gw.Handler())
}

func TestWorkspaceManager_MCPTools(t *testing.T) {
	srv := startStubGateway(t, func(gw *workspace.MCPGateway) {
		gw.RegisterServer("demo", &wsToolsStubMCP{})
	})
	defer srv.Close()

	root := t.TempDir()
	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := NewWorkspaceManager(root, "").WithDefaultMCPGateway(srv.URL, "secret")
	if err := wsMgr.AddMCP(context.Background(), storage, "u1", "a1", "s1", MCPRegistration{
		Name:       "demo",
		GatewayURL: srv.URL,
		Token:      "secret",
	}); err != nil {
		t.Fatal(err)
	}

	tools, err := wsMgr.MCPTools(context.Background(), storage, "u1", "a1", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name() != "mcp__demo__ping" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestServer_EnrichContextWithWorkspaceTools(t *testing.T) {
	srv := startStubGateway(t, func(gw *workspace.MCPGateway) {
		gw.RegisterServer("demo", &wsToolsStubMCP{})
	})
	defer srv.Close()

	root := t.TempDir()
	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	wsMgr := NewWorkspaceManager(root, "").WithDefaultMCPGateway(srv.URL, "secret")
	_ = wsMgr.AddMCP(context.Background(), storage, "u1", "a1", "s1", MCPRegistration{
		Name: "demo", GatewayURL: srv.URL, Token: "secret",
	})

	server := NewServer(&mockAgent{name: "t"})
	server.WithStorage(storage).WithWorkspaceManager(wsMgr)

	ctx := context.WithValue(context.Background(), service.ContextKeyUserID, "u1")
	enriched := server.enrichContextWithWorkspaceTools(ctx, "a1", "s1")
	if len(runcontext.Tools(enriched)) != 1 {
		t.Fatalf("expected 1 session tool, got %d", len(runcontext.Tools(enriched)))
	}
}

func TestRestoreAllMCPsToGateway(t *testing.T) {
	var mu sync.Mutex
	registered := make(map[string]workspace.MCPServerSpec)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/mcps":
			switch r.Method {
			case http.MethodGet:
				mu.Lock()
				list := make([]workspace.MCPServerSpec, 0, len(registered))
				for _, spec := range registered {
					list = append(list, spec)
				}
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(list)
			case http.MethodPost:
				var spec workspace.MCPServerSpec
				_ = json.NewDecoder(r.Body).Decode(&spec)
				mu.Lock()
				registered[spec.Name] = spec
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			}
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	wsDir := root + "/u1/a1/s1"
	if err := saveMCPFile(wsDir, map[string]MCPRegistration{
		"fs": {
			Name:       "fs",
			GatewayURL: srv.URL,
			Spec: workspace.MCPServerSpec{
				Name:       "fs",
				IsStateful: true,
				MCPConfig:  workspace.MCPConfigSpec{Type: "stdio_mcp", Command: "echo"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	wsMgr := NewWorkspaceManager(root, "").WithDefaultMCPGateway(srv.URL, "")
	if err := wsMgr.RestoreAllMCPsToGateway(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := registered["fs"]
	mu.Unlock()
	if got.Name != "fs" {
		t.Fatalf("expected fs restored, got %#v", got)
	}
}
