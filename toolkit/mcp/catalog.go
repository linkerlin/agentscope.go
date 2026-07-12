package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerSpec is a declarative description of one MCP server connection. It can
// be loaded from YAML so users add MCP servers via config, not Go code.
type ServerSpec struct {
	Name      string            `yaml:"name" json:"name"`
	Transport string            `yaml:"transport" json:"transport"` // stdio | sse | http
	Command   string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env       map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	URL       string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Enabled   *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"` // nil/true = enabled
	Timeout   time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// IsEnabled reports whether the spec should be connected (default true).
func (s ServerSpec) IsEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// CommonServers is a catalog of widely-used MCP servers, ready to copy into a
// config. Transport args reference the official @modelcontextprotocol npm
// packages (run via npx) and the @playwright browser server. Substitute real
// paths/tokens before use.
var CommonServers = map[string]ServerSpec{
	"filesystem": {
		Name: "filesystem", Transport: "stdio",
		Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "$WORKDIR"},
	},
	"fetch": {
		Name: "fetch", Transport: "stdio",
		Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-fetch"},
	},
	"playwright": {
		Name: "browser", Transport: "stdio",
		Command: "npx", Args: []string{"@playwright/mcp@latest"},
	},
	"github": {
		Name: "github", Transport: "stdio",
		Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_TOKEN"},
	},
	"sqlite": {
		Name: "sqlite", Transport: "stdio",
		Command: "uvx", Args: []string{"mcp-server-sqlite", "--db-path", "$DB_PATH"},
	},
	"brave-search": {
		Name: "brave-search", Transport: "stdio",
		Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-brave-search"},
		Env: map[string]string{"BRAVE_API_KEY": "$BRAVE_API_KEY"},
	},
}

// ExpandEnv substitutes $VAR / ${VAR} references in Command, Args, Env values,
// and URL using os.ExpandEnv. Lets configs reference secrets/paths without
// hardcoding them.
func (s *ServerSpec) ExpandEnv() {
	s.Command = os.ExpandEnv(s.Command)
	for i, a := range s.Args {
		s.Args[i] = os.ExpandEnv(a)
	}
	for k, v := range s.Env {
		s.Env[k] = os.ExpandEnv(v)
	}
	s.URL = os.ExpandEnv(s.URL)
}

// BuildClient constructs a Client from the spec (without connecting).
func (s ServerSpec) BuildClient() (Client, error) {
	b := NewClientBuilder(s.Name)
	switch s.Transport {
	case "stdio", "":
		if s.Command == "" {
			return nil, fmt.Errorf("mcp: server %q stdio transport requires command", s.Name)
		}
		b.StdioTransportWithEnv(s.Command, s.Env, s.Args...)
	case "sse":
		b.SSETransport(s.URL)
	case "http":
		b.StreamableHTTPTransport(s.URL)
	default:
		return nil, fmt.Errorf("mcp: server %q unknown transport %q", s.Name, s.Transport)
	}
	for k, v := range s.Headers {
		b.Header(k, v)
	}
	if s.Timeout > 0 {
		b.Timeout(s.Timeout)
	}
	return b.Build()
}

// ConnectResult records the outcome of connecting one server.
type ConnectResult struct {
	Spec  ServerSpec
	Err   error
	Tools int
}

// ConnectServers builds and connects each enabled ServerSpec, registering
// successful clients into a Manager. Servers that fail to connect (e.g. binary
// not installed) are skipped with a recorded error — the rest still work.
// This makes MCP wiring resilient: a missing optional server never breaks the
// agent.
func ConnectServers(ctx context.Context, specs []ServerSpec) (*Manager, []ConnectResult) {
	mgr := NewManager()
	var results []ConnectResult
	for _, spec := range specs {
		spec.ExpandEnv()
		if !spec.IsEnabled() {
			results = append(results, ConnectResult{Spec: spec, Err: fmt.Errorf("disabled")})
			continue
		}
		if spec.Transport == "stdio" && spec.Command != "" {
			if _, err := exec.LookPath(firstToken(spec.Command)); err != nil {
				results = append(results, ConnectResult{Spec: spec, Err: fmt.Errorf("binary not found: %w", err)})
				continue
			}
		}
		c, err := spec.BuildClient()
		if err != nil {
			results = append(results, ConnectResult{Spec: spec, Err: err})
			continue
		}
		if err := c.Connect(ctx, MCPConfig{Name: spec.Name}); err != nil {
			_ = c.Close()
			results = append(results, ConnectResult{Spec: spec, Err: err})
			continue
		}
		if err := mgr.Register(spec.Name, c); err != nil {
			_ = c.Close()
			results = append(results, ConnectResult{Spec: spec, Err: err})
			continue
		}
		n := 0
		if tools, err := c.ListTools(ctx); err == nil {
			n = len(tools)
		}
		results = append(results, ConnectResult{Spec: spec, Tools: n})
	}
	return mgr, results
}

// LoadSpecsFromYAML reads a ServerSpec list from a YAML file.
func LoadSpecsFromYAML(path string) ([]ServerSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcp: read %s: %w", path, err)
	}
	var doc struct {
		Servers []ServerSpec `yaml:"servers"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("mcp: parse %s: %w", path, err)
	}
	return doc.Servers, nil
}

// CloseManager closes all clients registered in a Manager. Best-effort.
func CloseManager(mgr *Manager) {
	if mgr == nil {
		return
	}
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	for _, c := range mgr.clients {
		_ = c.Close()
	}
}

func firstToken(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	return s
}
