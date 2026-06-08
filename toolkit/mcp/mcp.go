package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// MCPConfig MCP 连接描述（具体传输由 Client 实现解释）
type MCPConfig struct {
	Name string
	// Endpoint 等字段由具体 Client 实现使用
	Endpoint string
}

// ToolInfo MCP 工具元数据
type ToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema object
	ReadOnly    bool
	MCPName     string // MCP server label/name
}

// Client MCP 客户端抽象（可接 stdio、HTTP、mock）
type Client interface {
	Connect(ctx context.Context, cfg MCPConfig) error
	ListTools(ctx context.Context) ([]ToolInfo, error)
	CallTool(ctx context.Context, name string, args map[string]any) (any, error)
	Close() error
}

// ElicitRequest represents an MCP elicitation request.
// Exact fields depend on the MCP protocol version; this is a minimal stable subset.
type ElicitRequest struct {
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// ElicitResult represents the result of an MCP elicitation.
type ElicitResult struct {
	Accepted bool           `json:"accepted,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// ElicitationClient is an optional extension for MCP clients that support elicitation.
type ElicitationClient interface {
	Client
	Elicit(ctx context.Context, req ElicitRequest) (ElicitResult, error)
}

// Elicit tries to invoke elicitation on a client if it implements ElicitationClient.
func Elicit(ctx context.Context, c Client, req ElicitRequest) (ElicitResult, error) {
	if ec, ok := c.(ElicitationClient); ok {
		return ec.Elicit(ctx, req)
	}
	return ElicitResult{}, fmt.Errorf("mcp: client %T does not support elicitation", c)
}

// Manager 管理多个 MCP Client，并将工具适配为 tool.Tool
type Manager struct {
	mu      sync.RWMutex
	clients map[string]Client // 逻辑名 -> 客户端
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{clients: make(map[string]Client)}
}

// Register 注册已 Connect 的客户端（label 用于工具名前缀避免冲突）
func (m *Manager) Register(label string, c Client) error {
	if label == "" || c == nil {
		return fmt.Errorf("mcp: invalid label or client")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.clients[label]; ok {
		return fmt.Errorf("mcp: client already registered: %s", label)
	}
	m.clients[label] = c
	return nil
}

// Tools 列出所有已连接客户端暴露的工具并包装为 tool.Tool
func (m *Manager) Tools(ctx context.Context) ([]tool.Tool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []tool.Tool
	for label, c := range m.clients {
		infos, err := c.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("mcp list tools %s: %w", label, err)
		}
		for _, info := range infos {
			out = append(out, NewToolAdapter(label, c, info))
		}
	}
	return out, nil
}

type toolAdapter struct {
	label  string
	client Client
	info   ToolInfo
}

// FormatToolName builds a PyV2-compatible MCP tool name: mcp__{server}__{tool}.
func FormatToolName(server, tool string) string {
	return fmt.Sprintf("mcp__%s__%s", server, tool)
}

// ParseToolName splits mcp__{server}__{tool} into server and tool.
// Returns ok=false for legacy label/tool names.
func ParseToolName(full string) (server, tool string, ok bool) {
	const prefix = "mcp__"
	if !strings.HasPrefix(full, prefix) {
		return "", full, false
	}
	rest := full[len(prefix):]
	idx := strings.Index(rest, "__")
	if idx <= 0 || idx >= len(rest)-2 {
		return "", full, false
	}
	return rest[:idx], rest[idx+2:], true
}

// UnderlyingToolName returns the raw MCP tool name for enable/disable filters.
func UnderlyingToolName(full string) string {
	if _, tool, ok := ParseToolName(full); ok {
		return tool
	}
	if before, after, ok := strings.Cut(full, "/"); ok && before != "" && after != "" {
		return after
	}
	return full
}

// NewToolAdapter 将 MCP ToolInfo 包装为 tool.Tool
func NewToolAdapter(label string, c Client, info ToolInfo) tool.Tool {
	if info.MCPName == "" {
		info.MCPName = label
	}
	return &toolAdapter{label: label, client: c, info: info}
}

func (t *toolAdapter) Name() string {
	server := t.info.MCPName
	if server == "" {
		server = t.label
	}
	return FormatToolName(server, t.info.Name)
}

func (t *toolAdapter) Description() string { return t.info.Description }

func (t *toolAdapter) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.info.Description,
		Parameters:  t.info.Parameters,
	}
}

func (t *toolAdapter) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	result, err := t.client.CallTool(ctx, t.info.Name, input)
	if err != nil {
		return nil, err
	}
	return tool.NewTextResponse(result), nil
}

func (t *toolAdapter) IsMCPTool() bool { return true }

func (t *toolAdapter) MCPName() string {
	if t.info.MCPName != "" {
		return t.info.MCPName
	}
	return t.label
}

func (t *toolAdapter) IsReadOnly() bool { return t.info.ReadOnly }

func (t *toolAdapter) CheckPermissions(_ map[string]any, _ any) (tool.PermissionDecision, string, string, bool) {
	if t.info.ReadOnly {
		return tool.PermAllow, "This is a read-only MCP tool. Allowing execution.", "read-only mcp tool", false
	}
	return tool.PermAsk, "MCP tools must be explicitly allowed by the user.", "mcp tool default", false
}

func (t *toolAdapter) MatchRule(pattern string, _ map[string]any) bool {
	return pattern == ""
}

func (t *toolAdapter) GenerateSuggestions(_ map[string]any) []tool.SuggestedRule {
	return []tool.SuggestedRule{{
		Name:     "suggested-tool-level",
		ToolName: t.Name(),
		Target:   "tool_name",
		Pattern:  t.Name(),
		Decision: tool.PermAllow,
	}}
}
