package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/toolkit/mcp"
	"github.com/linkerlin/agentscope.go/workspace"
)

type stubGatewayMCP struct {
	tools []mcp.ToolInfo
}

func (s *stubGatewayMCP) Connect(context.Context, mcp.MCPConfig) error { return nil }
func (s *stubGatewayMCP) ListTools(context.Context) ([]mcp.ToolInfo, error) {
	return s.tools, nil
}
func (s *stubGatewayMCP) CallTool(context.Context, string, map[string]any) (any, error) {
	return nil, nil
}
func (s *stubGatewayMCP) Close() error { return nil }

func newTestMCPGateway(t *testing.T) (*workspace.MCPGateway, *httptest.Server) {
	t.Helper()
	gw := workspace.NewMCPGateway("secret")
	gw.RegisterServer("existing", &stubGatewayMCP{
		tools: []mcp.ToolInfo{{Name: "ping", Description: "ping"}},
	})
	return gw, httptest.NewServer(gw.Handler())
}

func TestWorkspace_MCPAttachListRemove(t *testing.T) {
	_, gatewaySrv := newTestMCPGateway(t)
	defer gatewaySrv.Close()

	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := NewWorkspaceManager(t.TempDir(), "")
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	q := "?agent_id=a1&session_id=s1"
	ctx := context.WithValue(context.Background(), service.ContextKeyUserID, "u1")

	attachBody, _ := json.Marshal(MCPRegistration{
		Name:       "existing",
		GatewayURL: gatewaySrv.URL,
		Token:      "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/workspace/mcp"+q, bytes.NewReader(attachBody))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("attach mcp: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/workspace/mcp"+q, nil)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list mcps: expected 200, got %d", rr.Code)
	}
	var listed []MCPStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || !listed[0].IsHealthy || listed[0].Name != "existing" {
		t.Fatalf("unexpected list: %#v", listed)
	}

	req = httptest.NewRequest(http.MethodDelete, "/workspace/mcp/existing"+q, nil)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete mcp: expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWorkspaceManager_AddMCP_PostsSpec(t *testing.T) {
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
				if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				mu.Lock()
				registered[spec.Name] = spec
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			if len(r.URL.Path) > len("/mcps/") && r.Method == http.MethodDelete {
				name := r.URL.Path[len("/mcps/"):]
				mu.Lock()
				delete(registered, name)
				mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer gatewaySrv.Close()

	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := NewWorkspaceManager(t.TempDir(), "").
		WithDefaultMCPGateway(gatewaySrv.URL, "")

	spec := workspace.MCPServerSpec{
		Name:       "fs",
		IsStateful: true,
		MCPConfig: workspace.MCPConfigSpec{
			Type:    "stdio_mcp",
			Command: "echo",
		},
	}
	err := wsMgr.AddMCP(context.Background(), storage, "u1", "a1", "s1", MCPRegistration{
		GatewayURL: gatewaySrv.URL,
		Spec:       spec,
	})
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	got := registered["fs"]
	mu.Unlock()
	if got.Name != "fs" || got.MCPConfig.Command != "echo" {
		t.Fatalf("gateway did not receive spec: %#v", got)
	}

	if err := wsMgr.RemoveMCP(context.Background(), storage, "u1", "a1", "s1", "fs"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	_, ok := registered["fs"]
	mu.Unlock()
	if ok {
		t.Fatal("expected fs removed from mock gateway")
	}
}

func TestWorkspace_MCPPostWithDefaultGateway(t *testing.T) {
	var posted workspace.MCPServerSpec
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/mcps" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &posted)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer gatewaySrv.Close()

	storage := service.NewMemoryStorage()
	_ = storage.SaveSession(context.Background(), &service.Session{
		ID: "s1", UserID: "u1", AgentID: "a1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	wsMgr := NewWorkspaceManager(t.TempDir(), "").
		WithDefaultMCPGateway(gatewaySrv.URL, "")
	srv := NewServer(&mockAgent{name: "test"})
	srv.WithStorage(storage).WithWorkspaceManager(wsMgr).RegisterWorkspaceRoutes()

	q := "?agent_id=a1&session_id=s1"
	ctx := context.WithValue(context.Background(), service.ContextKeyUserID, "u1")
	addBody := []byte(`{"name":"fs","is_stateful":true,"mcp_config":{"type":"stdio_mcp","command":"echo"}}`)
	req := httptest.NewRequest(http.MethodPost, "/workspace/mcp"+q, bytes.NewReader(addBody))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if posted.Name != "fs" || posted.MCPConfig.Type != "stdio_mcp" {
		t.Fatalf("unexpected posted spec: %#v", posted)
	}
}

func TestParseMCPAddRequest_DefaultGateway(t *testing.T) {
	body := []byte(`{"name":"demo","is_stateful":true,"mcp_config":{"type":"stdio_mcp","command":"echo"}}`)
	reg, err := parseMCPAddRequest(body, "http://gw", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if reg.GatewayURL != "http://gw" || reg.Token != "tok" || reg.Spec.MCPConfig.Command != "echo" {
		t.Fatalf("unexpected reg: %#v", reg)
	}
}
