package toolkit

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/toolkit/mcp"
)

type mcpRegisterConfig struct {
	groupName    string
	enableTools  []string
	disableTools []string
}

// MCPRegisterOption 配置 MCP 客户端注册行为
type MCPRegisterOption func(*mcpRegisterConfig)

// WithMCPGroup 将 MCP 工具加入指定分组
func WithMCPGroup(name string) MCPRegisterOption {
	return func(cfg *mcpRegisterConfig) {
		cfg.groupName = name
	}
}

// WithMCPEnableTools 仅启用指定工具（白名单）
func WithMCPEnableTools(names ...string) MCPRegisterOption {
	return func(cfg *mcpRegisterConfig) {
		cfg.enableTools = names
	}
}

// WithMCPDisableTools 禁用指定工具（黑名单）
func WithMCPDisableTools(names ...string) MCPRegisterOption {
	return func(cfg *mcpRegisterConfig) {
		cfg.disableTools = names
	}
}

// RegisterMCPClient 将 MCP 客户端的工具注册到 Toolkit
func (tk *Toolkit) RegisterMCPClient(ctx context.Context, label string, c mcp.Client, opts ...MCPRegisterOption) error {
	tools, err := c.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("toolkit: list mcp tools %s: %w", label, err)
	}

	cfg := &mcpRegisterConfig{}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.groupName != "" && !tk.Groups.HasGroup(cfg.groupName) {
		if err := tk.Groups.CreateGroup(cfg.groupName, ""); err != nil {
			return err
		}
	}

	for _, info := range tools {
		if !shouldRegisterMCP(info.Name, cfg.enableTools, cfg.disableTools) {
			continue
		}
		ta := mcp.NewToolAdapter(label, c, info)
		if err := tk.Registry.Register(ta); err != nil {
			return fmt.Errorf("toolkit: register mcp tool %s: %w", ta.Name(), err)
		}
		if cfg.groupName != "" {
			if err := tk.Groups.AddTool(cfg.groupName, ta.Name()); err != nil {
				return fmt.Errorf("toolkit: add mcp tool to group %s: %w", cfg.groupName, err)
			}
		}
	}
	return nil
}

func shouldRegisterMCP(toolName string, enable, disable []string) bool {
	result := true
	if len(disable) > 0 {
		for _, d := range disable {
			if d == toolName {
				result = false
				break
			}
		}
	}
	if len(enable) > 0 {
		result = false
		for _, e := range enable {
			if e == toolName {
				result = true
				break
			}
		}
	}
	return result
}
