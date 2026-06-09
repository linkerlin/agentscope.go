package workspace

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// Endpoints mirror PyV2 workspace/_mcp_gateway/_mcp_gateway_app.py.
type mcpServerEntry struct {
	client mcp.Client
	spec   MCPServerSpec
}

// MCPGateway is a lightweight in-process HTTP proxy for MCP tools inside a sandbox.
type MCPGateway struct {
	mu      sync.RWMutex
	token   string
	servers map[string]mcpServerEntry
	mux     *http.ServeMux
}

// NewMCPGateway creates an MCP gateway with optional bearer token auth.
func NewMCPGateway(token string) *MCPGateway {
	g := &MCPGateway{
		token:   token,
		servers: make(map[string]mcpServerEntry),
		mux:     http.NewServeMux(),
	}
	g.mux.HandleFunc("/health", g.handleHealth)
	g.mux.HandleFunc("/mcps", g.handleMCPs)
	g.mux.HandleFunc("/mcps/", g.handleMCPTool)
	return g
}

// RegisterServer adds or replaces an MCP client under name.
// The returned GET /mcps spec contains at least name/is_stateful for test stubs.
func (g *MCPGateway) RegisterServer(name string, client mcp.Client) {
	g.registerServer(name, client, MCPServerSpec{
		Name:       name,
		IsStateful: true,
		MCPConfig:  MCPConfigSpec{Type: "stdio_mcp"},
	})
}

func (g *MCPGateway) registerServer(name string, client mcp.Client, spec MCPServerSpec) {
	if spec.Name == "" {
		spec.Name = name
	}
	if !spec.IsStateful {
		spec.IsStateful = true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.servers[name] = mcpServerEntry{client: client, spec: spec}
}

// Handler returns the HTTP handler for the gateway.
func (g *MCPGateway) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.token != "" && r.URL.Path != "/health" {
			if r.Header.Get("Authorization") != "Bearer "+g.token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		g.mux.ServeHTTP(w, r)
	})
}

func (g *MCPGateway) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

func (g *MCPGateway) handleMCPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.mu.RLock()
		list := make([]MCPServerSpec, 0, len(g.servers))
		for _, entry := range g.servers {
			list = append(list, entry.spec)
		}
		g.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req, spec, err := ParseMCPRegisterBody(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g.mu.RLock()
		_, exists := g.servers[req.Name]
		g.mu.RUnlock()
		if exists {
			http.Error(w, req.Name+" already exists", http.StatusConflict)
			return
		}
		if err := g.RegisterClientFromRequest(r.Context(), req, spec); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (g *MCPGateway) handleMCPTool(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	if len(parts) < 2 || parts[0] != "mcps" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	name := parts[1]

	// DELETE /mcps/{name}
	if len(parts) == 2 && r.Method == http.MethodDelete {
		g.mu.Lock()
		entry, ok := g.servers[name]
		delete(g.servers, name)
		g.mu.Unlock()
		if !ok {
			http.Error(w, "mcp server not found", http.StatusNotFound)
			return
		}
		_ = entry.client.Close()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// GET /mcps/{name}/tools
	if len(parts) == 3 && parts[2] == "tools" && r.Method == http.MethodGet {
		g.mu.RLock()
		entry, ok := g.servers[name]
		g.mu.RUnlock()
		if !ok {
			http.Error(w, "mcp server not found", http.StatusNotFound)
			return
		}
		tools, err := entry.client.ListTools(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tools)
		return
	}

	// POST /mcps/{name}/tools/{tool}
	if len(parts) < 4 || parts[2] != "tools" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	toolName := parts[3]
	g.mu.RLock()
	entry, ok := g.servers[name]
	g.mu.RUnlock()
	if !ok {
		http.Error(w, "mcp server not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := entry.client.CallTool(r.Context(), toolName, body.Arguments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func splitPath(p string) []string {
	var out []string
	cur := ""
	for _, ch := range p {
		if ch == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
