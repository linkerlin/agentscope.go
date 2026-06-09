package gateway

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/runcontext"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/workspace"
)

// MCPTools returns gateway-backed MCP tools registered on the session workspace.
func (m *WorkspaceManager) MCPTools(ctx context.Context, st service.Storage, userID, agentID, sessionID string) ([]tool.Tool, error) {
	sw, err := m.GetOrCreate(ctx, st, userID, agentID, sessionID)
	if err != nil {
		return nil, err
	}
	var out []tool.Tool
	for _, reg := range sw.MCP {
		reg, err := m.applyMCPDefaults(ctx, sessionID, sw.dir, reg)
		if err != nil {
			return nil, err
		}
		client := workspace.NewGatewayClient(reg.GatewayURL, reg.Token)
		gwClient := client.MakeMCPClient(reg.Name)
		infos, err := gwClient.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("list mcp tools %q: %w", reg.Name, err)
		}
		for _, info := range infos {
			out = append(out, workspace.NewGatewayMCPTool(reg.Name, info, client))
		}
	}
	return out, nil
}

// RestoreAllMCPsToGateway re-registers MCP specs from every session .mcp file onto gateways.
// With per-session gateways each session is restored onto its own sidecar.
func (m *WorkspaceManager) RestoreAllMCPsToGateway(ctx context.Context) error {
	if m == nil || m.baseRoot == "" {
		return nil
	}
	if m.perSessionGateway {
		return m.restoreAllMCPsPerSession(ctx)
	}
	if m.defaultMCPGatewayURL == "" {
		return nil
	}
	client := workspace.NewGatewayClient(m.defaultMCPGatewayURL, m.defaultMCPGatewayToken)
	if err := client.Health(ctx); err != nil {
		return fmt.Errorf("mcp gateway unhealthy: %w", err)
	}
	existing, _ := client.ListMCPs(ctx)
	known := make(map[string]bool, len(existing))
	for _, spec := range existing {
		known[spec.Name] = true
	}

	return filepath.WalkDir(m.baseRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != mcpPersistFile {
			return nil
		}
		regs, err := loadMCPFile(filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, reg := range regs {
			reg, err := m.applyMCPDefaults(ctx, "", filepath.Dir(path), reg)
			if err != nil {
				continue
			}
			if reg.Spec.MCPConfig.Type == "" || known[reg.Name] {
				continue
			}
			spec := reg.Spec
			if spec.Name == "" {
				spec.Name = reg.Name
			}
			if !spec.IsStateful {
				spec.IsStateful = true
			}
			if err := client.RegisterMCP(ctx, spec); err != nil {
				continue
			}
			known[reg.Name] = true
		}
		return nil
	})
}

func (m *WorkspaceManager) restoreAllMCPsPerSession(ctx context.Context) error {
	return filepath.WalkDir(m.baseRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != mcpPersistFile {
			return nil
		}
		wsDir := filepath.Dir(path)
		sessionID := filepath.Base(wsDir)
		regs, err := loadMCPFile(wsDir)
		if err != nil {
			return err
		}
		for _, reg := range regs {
			if _, syncErr := m.syncMCPToGateway(ctx, sessionID, wsDir, reg); syncErr != nil {
				continue
			}
		}
		return nil
	})
}

func (s *Server) enrichContextWithWorkspaceTools(ctx context.Context, agentID, sessionID string) context.Context {
	if s == nil || s.workspaceMgr == nil || s.storage == nil || sessionID == "" {
		return ctx
	}
	userID := service.UserIDFromContext(ctx)
	if userID == "" || agentID == "" {
		if se, err := s.storage.GetSession(ctx, sessionID); err == nil && se != nil {
			if userID == "" {
				userID = se.UserID
			}
			if agentID == "" {
				agentID = se.AgentID
			}
		}
	}
	if userID == "" || agentID == "" {
		return ctx
	}
	tools, err := s.workspaceMgr.MCPTools(ctx, s.storage, userID, agentID, sessionID)
	if err != nil || len(tools) == 0 {
		return ctx
	}
	return runcontext.WithTools(ctx, tools)
}
