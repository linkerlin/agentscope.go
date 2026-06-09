package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/linkerlin/agentscope.go/workspace"
)

const mcpPersistFile = ".mcp"

func mcpFilePath(wsDir string) string {
	return filepath.Join(wsDir, mcpPersistFile)
}

func saveMCPFile(wsDir string, mcps map[string]MCPRegistration) error {
	if wsDir == "" {
		return nil
	}
	names := make([]string, 0, len(mcps))
	for name := range mcps {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]MCPRegistration, 0, len(names))
	for _, name := range names {
		list = append(list, mcps[name])
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(mcpFilePath(wsDir), data, 0o644)
}

func loadMCPFile(wsDir string) ([]MCPRegistration, error) {
	path := mcpFilePath(wsDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var list []MCPRegistration
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", mcpPersistFile, err)
	}
	return list, nil
}

func (m *WorkspaceManager) applyMCPDefaults(ctx context.Context, sessionID, wsDir string, reg MCPRegistration) (MCPRegistration, error) {
	if reg.Name == "" {
		reg.Name = reg.Spec.Name
	}
	if m.perSessionGateway && sessionID != "" && m.sessionGateways != nil {
		url, token, err := m.sessionGateways.Ensure(ctx, sessionID)
		if err != nil {
			return reg, err
		}
		reg.GatewayURL = url
		reg.Token = token
	} else {
		if reg.GatewayURL == "" {
			reg.GatewayURL = m.defaultMCPGatewayURL
		}
		if reg.Token == "" {
			reg.Token = m.defaultMCPGatewayToken
		}
	}
	if reg.Spec.Name == "" {
		reg.Spec.Name = reg.Name
	}
	_ = wsDir
	return reg, nil
}

func (m *WorkspaceManager) syncMCPToGateway(ctx context.Context, sessionID, wsDir string, reg MCPRegistration) (MCPRegistration, error) {
	var err error
	reg, err = m.applyMCPDefaults(ctx, sessionID, wsDir, reg)
	if err != nil {
		return reg, err
	}
	if reg.Name == "" || reg.GatewayURL == "" {
		return reg, fmt.Errorf("name and gateway_url are required")
	}
	client := workspace.NewGatewayClient(reg.GatewayURL, reg.Token)
	if err := client.Health(ctx); err != nil {
		return reg, fmt.Errorf("mcp gateway unhealthy: %w", err)
	}
	if reg.Spec.MCPConfig.Type != "" {
		spec := reg.Spec
		if spec.Name == "" {
			spec.Name = reg.Name
		}
		if !spec.IsStateful {
			spec.IsStateful = true
		}
		if err := client.RegisterMCP(ctx, spec); err != nil {
			return reg, fmt.Errorf("gateway register mcp: %w", err)
		}
		reg.Spec = spec
	} else if err := ensureMCPOnGateway(ctx, client, reg.Name); err != nil {
		return reg, err
	}
	return reg, nil
}

func (m *WorkspaceManager) restoreMCPs(ctx context.Context, sessionID, wsDir string, sw *SessionWorkspace) error {
	regs, err := loadMCPFile(wsDir)
	if err != nil || len(regs) == 0 {
		return err
	}
	for _, reg := range regs {
		reg, err := m.applyMCPDefaults(ctx, sessionID, wsDir, reg)
		if err != nil || reg.Name == "" {
			continue
		}
		synced, syncErr := m.syncMCPToGateway(ctx, sessionID, wsDir, reg)
		if syncErr == nil {
			reg = synced
		}
		sw.MCP[reg.Name] = reg
	}
	return nil
}
