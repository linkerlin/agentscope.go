package workspace

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

// MCPGateway is a lightweight in-process HTTP proxy for MCP tools inside a sandbox.
// Endpoints mirror PyV2 workspace/_mcp_gateway/_mcp_gateway_app.py.
type MCPGateway struct {
	mu      sync.RWMutex
	token   string
	servers map[string]mcp.Client
	mux     *http.ServeMux
}

// NewMCPGateway creates an MCP gateway with optional bearer token auth.
func NewMCPGateway(token string) *MCPGateway {
	g := &MCPGateway{
		token:   token,
		servers: make(map[string]mcp.Client),
		mux:     http.NewServeMux(),
	}
	g.mux.HandleFunc("/health", g.handleHealth)
	g.mux.HandleFunc("/mcps", g.handleMCPs)
	g.mux.HandleFunc("/mcps/", g.handleMCPTool)
	return g
}

// RegisterServer adds or replaces an MCP client under name.
func (g *MCPGateway) RegisterServer(name string, client mcp.Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.servers[name] = client
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
		type item struct {
			Name  string `json:"name"`
			Tools int    `json:"tools"`
		}
		var list []item
		for name, c := range g.servers {
			tools, _ := c.ListTools(r.Context())
			list = append(list, item{Name: name, Tools: len(tools)})
		}
		g.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
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
		delete(g.servers, name)
		g.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// GET /mcps/{name}/tools
	if len(parts) == 3 && parts[2] == "tools" && r.Method == http.MethodGet {
		g.mu.RLock()
		client, ok := g.servers[name]
		g.mu.RUnlock()
		if !ok {
			http.Error(w, "mcp server not found", http.StatusNotFound)
			return
		}
		tools, err := client.ListTools(r.Context())
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
	client, ok := g.servers[name]
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
	result, err := client.CallTool(r.Context(), toolName, body.Arguments)
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
