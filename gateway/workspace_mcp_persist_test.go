package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
	"github.com/linkerlin/agentscope.go/workspace"
)

func TestSaveLoadMCPFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	mcps := map[string]MCPRegistration{
		"b": {Name: "b", GatewayURL: "http://gw", Spec: workspace.MCPServerSpec{Name: "b"}},
		"a": {Name: "a", GatewayURL: "http://gw", Spec: workspace.MCPServerSpec{Name: "a"}},
	}
	if err := saveMCPFile(dir, mcps); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".mcp"))
	if err != nil {
		t.Fatal(err)
	}
	var list []MCPRegistration
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("expected stable sorted persist, got %s", raw)
	}
	loaded, err := loadMCPFile(dir)
	if err != nil || len(loaded) != 2 {
		t.Fatalf("load: err=%v len=%d", err, len(loaded))
	}
}

func TestWorkspaceManager_RestoresMCPFromFile(t *testing.T) {
	var mu sync.Mutex
	registered := make(map[string]workspace.MCPServerSpec)
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				gw := workspace.NewMCPGateway("")
				gw.RegisterServer(spec.Name, &persistStubMCP{
					tools: []mcp.ToolInfo{{Name: "tool1", Description: "d1"}},
				})
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			if len(r.URL.Path) > len("/mcps/") && r.URL.Path[len("/mcps/"):] != "" {
				parts := r.URL.Path[len("/mcps/"):]
				if parts != "" && len(parts) > len("tools") {
					_ = json.NewEncoder(w).Encode([]mcp.ToolInfo{{Name: "tool1", Description: "d1"}})
					return
				}
			}
			http.NotFound(w, r)
		}
	}))
	defer gatewaySrv.Close()

	root := t.TempDir()
	wsDir := filepath.Join(root, "u1", "a1", "s1")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := workspace.MCPServerSpec{
		Name:       "fs",
		IsStateful: true,
		MCPConfig:  workspace.MCPConfigSpec{Type: "stdio_mcp", Command: "echo"},
	}
	if err := saveMCPFile(wsDir, map[string]MCPRegistration{
		"fs": {
			Name:       "fs",
			GatewayURL: gatewaySrv.URL,
			Spec:       spec,
		},
	}); err != nil {
		t.Fatal(err)
	}

	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := NewWorkspaceManager(root, "")
	list, err := wsMgr.ListMCPs(context.Background(), storage, "u1", "a1", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "fs" {
		t.Fatalf("expected restored mcp, got %#v", list)
	}

	mu.Lock()
	got := registered["fs"]
	mu.Unlock()
	if got.Name != "fs" {
		t.Fatalf("expected gateway re-register on restore, got %#v", got)
	}
}

type persistStubMCP struct {
	tools []mcp.ToolInfo
}

func (s *persistStubMCP) Connect(context.Context, mcp.MCPConfig) error { return nil }
func (s *persistStubMCP) ListTools(context.Context) ([]mcp.ToolInfo, error) {
	return s.tools, nil
}
func (s *persistStubMCP) CallTool(context.Context, string, map[string]any) (any, error) {
	return nil, nil
}
func (s *persistStubMCP) Close() error { return nil }

func TestWorkspace_AddMCP_PersistsToDisk(t *testing.T) {
	var mu sync.Mutex
	registered := make(map[string]workspace.MCPServerSpec)
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/mcps" && r.Method == http.MethodPost {
			var spec workspace.MCPServerSpec
			_ = json.NewDecoder(r.Body).Decode(&spec)
			mu.Lock()
			registered[spec.Name] = spec
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer gatewaySrv.Close()

	root := t.TempDir()
	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	wsMgr := NewWorkspaceManager(root, "").WithDefaultMCPGateway(gatewaySrv.URL, "")

	err := wsMgr.AddMCP(context.Background(), storage, "u1", "a1", "s1", MCPRegistration{
		Spec: workspace.MCPServerSpec{
			Name:       "demo",
			IsStateful: true,
			MCPConfig:  workspace.MCPConfigSpec{Type: "stdio_mcp", Command: "echo"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := loadMCPFile(filepath.Join(root, "u1", "a1", "s1"))
	if err != nil || len(loaded) != 1 || loaded[0].Name != "demo" {
		t.Fatalf("persist file: err=%v loaded=%#v", err, loaded)
	}

	wsMgr2 := NewWorkspaceManager(root, "").WithDefaultMCPGateway(gatewaySrv.URL, "")
	list, err := wsMgr2.ListMCPs(context.Background(), storage, "u1", "a1", "s1")
	if err != nil || len(list) != 1 || list[0].Name != "demo" {
		t.Fatalf("restore via new manager: err=%v list=%#v", err, list)
	}
}
