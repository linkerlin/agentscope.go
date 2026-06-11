package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/skill"
	"github.com/linkerlin/agentscope.go/workspace"
)

// SessionWorkspace bundles filesystem workspace, skills and MCP registrations.
type SessionWorkspace struct {
	Workspace workspace.Workspace
	Skills    *skill.Registry
	MCP       map[string]MCPRegistration
	dir       string // session root; hosts .mcp persistence file
}

// Dir returns the session workspace root directory.
func (sw *SessionWorkspace) Dir() string {
	if sw == nil {
		return ""
	}
	return sw.dir
}

// MCPRegistration describes a host-side MCP gateway attachment.
// When Spec contains mcp_config, AddMCP POSTs the spec to the gateway.
type MCPRegistration struct {
	Name       string                  `json:"name"`
	GatewayURL string                  `json:"gateway_url"`
	Token      string                  `json:"token,omitempty"`
	Spec       workspace.MCPServerSpec `json:"spec,omitempty"`
}

// MCPStatus is returned by workspace list endpoints (PyV2 MCPClientStatus compatible).
type MCPStatus struct {
	workspace.MCPServerSpec
	GatewayURL string             `json:"gateway_url,omitempty"`
	IsHealthy  bool               `json:"is_healthy"`
	Tools      []workspaceMCPTool `json:"tools,omitempty"`
}

type workspaceMCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WorkspaceManager resolves per-session workspaces for HTTP management APIs.
type WorkspaceManager struct {
	mu                     sync.RWMutex
	sessions               map[string]*SessionWorkspace
	baseRoot               string
	skillsRoot             string
	defaultMCPGatewayURL   string
	defaultMCPGatewayToken string
	perSessionGateway      bool
	sessionGateways        *SessionMCPGatewayPool
}

// NewWorkspaceManager creates a manager with optional roots for local workspaces/skills.
func NewWorkspaceManager(baseRoot, skillsRoot string) *WorkspaceManager {
	return &WorkspaceManager{
		sessions:   make(map[string]*SessionWorkspace),
		baseRoot:   baseRoot,
		skillsRoot: skillsRoot,
	}
}

// WithDefaultMCPGateway sets the gateway used when POST /workspace/mcp omits gateway_url.
func (m *WorkspaceManager) WithDefaultMCPGateway(url, token string) *WorkspaceManager {
	m.defaultMCPGatewayURL = url
	m.defaultMCPGatewayToken = token
	return m
}

// WithPerSessionMCPGateway gives each session its own MCP gateway sidecar (avoids name conflicts).
func (m *WorkspaceManager) WithPerSessionMCPGateway(token string) *WorkspaceManager {
	m.perSessionGateway = true
	m.sessionGateways = NewSessionMCPGatewayPool(token)
	return m
}

// PerSessionMCPGateway reports whether sessions use isolated MCP gateways.
func (m *WorkspaceManager) PerSessionMCPGateway() bool {
	return m != nil && m.perSessionGateway
}

func workspaceKey(userID, agentID, sessionID string) string {
	return userID + "|" + agentID + "|" + sessionID
}

// CloseAll closes all tracked workspaces and per-session MCP gateways.
func (m *WorkspaceManager) CloseAll() {
	m.mu.Lock()
	for _, sw := range m.sessions {
		if sw.Workspace != nil {
			_ = sw.Workspace.Close()
		}
	}
	m.sessions = make(map[string]*SessionWorkspace)
	pool := m.sessionGateways
	m.mu.Unlock()
	if pool != nil {
		_ = pool.CloseAll(context.Background())
	}
}

// GetOrCreate returns the session workspace, creating a local workspace when missing.
func (m *WorkspaceManager) GetOrCreate(ctx context.Context, st service.Storage, userID, agentID, sessionID string) (*SessionWorkspace, error) {
	if err := m.ensureSession(ctx, st, userID, agentID, sessionID); err != nil {
		return nil, err
	}
	key := workspaceKey(userID, agentID, sessionID)
	m.mu.RLock()
	sw := m.sessions[key]
	m.mu.RUnlock()
	if sw == nil {
		return nil, fmt.Errorf("workspace not found for session %s", sessionID)
	}
	return sw, nil
}

func (m *WorkspaceManager) ensureSession(ctx context.Context, st service.Storage, userID, agentID, sessionID string) error {
	if st == nil {
		return fmt.Errorf("storage not configured")
	}
	se, err := st.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if se.UserID != userID || se.AgentID != agentID {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	key := workspaceKey(userID, agentID, sessionID)
	m.mu.RLock()
	_, ok := m.sessions[key]
	m.mu.RUnlock()
	if ok {
		return nil
	}

	root := m.baseRoot
	if root == "" {
		root = os.TempDir()
	}
	wsDir := filepath.Join(root, userID, agentID, sessionID)
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return err
	}
	sw := &SessionWorkspace{
		Workspace: workspace.NewLocalWorkspace(sessionID, wsDir),
		Skills:    skill.NewRegistry(),
		MCP:       make(map[string]MCPRegistration),
		dir:       wsDir,
	}
	_ = m.restoreMCPs(ctx, sessionID, wsDir, sw)
	m.mu.Lock()
	m.sessions[key] = sw
	m.mu.Unlock()
	return nil
}

// ListSkills returns registered skills for a session workspace.
func (m *WorkspaceManager) ListSkills(ctx context.Context, st service.Storage, userID, agentID, sessionID string) ([]*skill.AgentSkill, error) {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return nil, err
	}
	return sw.Skills.List(), nil
}

// AddSkill loads a skill from path and registers it on the session workspace.
func (m *WorkspaceManager) AddSkill(ctx context.Context, st service.Storage, userID, agentID, sessionID, skillPath string) error {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return err
	}
	s, err := loadSkillFromPath(m.skillsRoot, skillPath)
	if err != nil {
		return err
	}
	sw.Skills.Register(s)
	return nil
}

// RemoveSkill removes a skill by display name or skill ID.
func (m *WorkspaceManager) RemoveSkill(ctx context.Context, st service.Storage, userID, agentID, sessionID, skillName string) error {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return err
	}
	for _, s := range sw.Skills.List() {
		if s.Name == skillName || s.SkillID() == skillName {
			sw.Skills.Remove(s.SkillID())
			return nil
		}
	}
	return fmt.Errorf("skill not found: %s", skillName)
}

// ListMCPs returns MCP registrations with live health/tool metadata.
func (m *WorkspaceManager) ListMCPs(ctx context.Context, st service.Storage, userID, agentID, sessionID string) ([]MCPStatus, error) {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]MCPStatus, 0, len(sw.MCP))
	for _, reg := range sw.MCP {
		reg, _ = m.applyMCPDefaults(ctx, sessionID, sw.dir, reg)
		status := MCPStatus{
			MCPServerSpec: reg.Spec,
			GatewayURL:    reg.GatewayURL,
		}
		if status.Name == "" {
			status.Name = reg.Name
		}
		client := workspace.NewGatewayClient(reg.GatewayURL, reg.Token)
		if err := client.Health(ctx); err != nil {
			out = append(out, status)
			continue
		}
		status.IsHealthy = true
		tools, err := client.MakeMCPClient(reg.Name).ListTools(ctx)
		if err == nil {
			for _, t := range tools {
				status.Tools = append(status.Tools, workspaceMCPTool{Name: t.Name, Description: t.Description})
			}
		}
		out = append(out, status)
	}
	return out, nil
}

// AddMCP registers an MCP on the session workspace and optionally POSTs spec to the gateway.
func (m *WorkspaceManager) AddMCP(ctx context.Context, st service.Storage, userID, agentID, sessionID string, reg MCPRegistration) error {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return err
	}
	reg, err = m.applyMCPDefaults(ctx, sessionID, sw.dir, reg)
	if err != nil {
		return err
	}
	if reg.Name == "" || reg.GatewayURL == "" {
		return fmt.Errorf("name and gateway_url are required")
	}
	if _, exists := sw.MCP[reg.Name]; exists {
		return fmt.Errorf("mcp %q already exists in workspace", reg.Name)
	}

	reg, err = m.syncMCPToGateway(ctx, sessionID, sw.dir, reg)
	if err != nil {
		return err
	}

	sw.MCP[reg.Name] = reg
	return saveMCPFile(sw.dir, sw.MCP)
}

// RemoveMCP deletes an MCP registration and unregisters it from the gateway when applicable.
func (m *WorkspaceManager) RemoveMCP(ctx context.Context, st service.Storage, userID, agentID, sessionID, name string) error {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return err
	}
	reg, ok := sw.MCP[name]
	if !ok {
		return fmt.Errorf("mcp not found: %s", name)
	}
	client := workspace.NewGatewayClient(reg.GatewayURL, reg.Token)
	if err := client.RemoveMCP(ctx, name); err != nil {
		// Best-effort: gateway may already have removed the MCP.
		if reg.Spec.MCPConfig.Type != "" {
			return fmt.Errorf("gateway remove mcp: %w", err)
		}
	}
	delete(sw.MCP, name)
	return saveMCPFile(sw.dir, sw.MCP)
}

func ensureMCPOnGateway(ctx context.Context, client *workspace.GatewayClient, name string) error {
	specs, err := client.ListMCPs(ctx)
	if err != nil {
		return fmt.Errorf("gateway list mcps: %w", err)
	}
	for _, spec := range specs {
		if spec.Name == name {
			return nil
		}
	}
	return fmt.Errorf("mcp %q not registered on gateway", name)
}

func loadSkillFromPath(skillsRoot, skillPath string) (*skill.AgentSkill, error) {
	if skillPath == "" {
		return nil, fmt.Errorf("skill_path is required")
	}
	candidates := []struct {
		root string
		name string
	}{}
	if skillsRoot != "" {
		candidates = append(candidates, struct {
			root string
			name string
		}{root: skillsRoot, name: skillPath})
	}
	if st, err := os.Stat(skillPath); err == nil && st.IsDir() {
		if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err == nil {
			candidates = append(candidates, struct {
				root string
				name string
			}{root: filepath.Dir(skillPath), name: filepath.Base(skillPath)})
		}
	}
	for _, c := range candidates {
		skillMD := filepath.Join(c.root, c.name, "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			continue
		}
		repo := skill.NewFileSystemRepository(c.root)
		return repo.GetSkill(c.name)
	}
	return nil, fmt.Errorf("skill path not found: %s", skillPath)
}
