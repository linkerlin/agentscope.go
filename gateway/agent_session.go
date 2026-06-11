package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/skill"
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	scheduletool "github.com/linkerlin/agentscope.go/tool/schedule"
	"github.com/linkerlin/agentscope.go/tool/shell"
	tasktool "github.com/linkerlin/agentscope.go/tool/task"
	"github.com/linkerlin/agentscope.go/toolkit"
	"github.com/linkerlin/agentscope.go/workspace"
)

// SessionAgentDeps bundles optional shared tools for per-session agents.
type SessionAgentDeps struct {
	TaskStore      *state.TaskStore
	ScheduleMgr    scheduletool.Manager
	TaskStop       tool.Tool
	ToolOffload    *ToolOffloadManager
	ExtraTools     []tool.Tool
	DefaultPrompt  string
	PermissionMode permission.Mode
}

// SessionAgentBuilder builds an agent for a specific session (Py get_agent parity).
type SessionAgentBuilder func(ctx context.Context, userID, agentID, sessionID string) (agent.Agent, error)

// WithSessionAgentBuilder enables per-session agent resolution during chat.
func (s *Server) WithSessionAgentBuilder(fn SessionAgentBuilder) *Server {
	s.sessionAgentBuilder = fn
	return s
}

// BuildSessionAgent assembles a ReAct agent bound to a session workspace.
func (f *AgentFactory) BuildSessionAgent(
	cfg *service.AgentConfig,
	cred *service.Credential,
	sw *SessionWorkspace,
	deps SessionAgentDeps,
) (agent.Agent, error) {
	if sw == nil || sw.Workspace == nil {
		return nil, fmt.Errorf("agent_factory: session workspace required")
	}

	chatModel, err := f.buildModel(cfg, cred)
	if err != nil {
		return nil, err
	}

	wsDir := sw.dir
	if wsDir == "" {
		return nil, fmt.Errorf("agent_factory: session workspace dir missing")
	}

	name := "session-agent"
	sysPrompt := deps.DefaultPrompt
	if cfg != nil {
		if cfg.Name != "" {
			name = cfg.Name
		}
		if cfg.SystemPrompt != "" {
			sysPrompt = cfg.SystemPrompt
		}
	}
	if sysPrompt == "" {
		sysPrompt = "You are a helpful assistant with access to a session workspace."
	}

	mode := deps.PermissionMode
	if mode == "" {
		mode = permission.ModeExplore
	}
	permEngine := permission.NewEngine(mode, defaultWorkspacePermRules())

	tk := toolkit.NewToolkit()
	for _, t := range sessionWorkspaceTools(wsDir, deps) {
		if err := tk.Register(t); err != nil {
			return nil, err
		}
	}
	if sw.Skills != nil {
		_, skillHook, err := skill.RegisterWithToolkit(tk, sw.Skills, skill.AttachOptions{})
		if err != nil {
			return nil, err
		}
		offloader := workspace.NewWorkspaceOffloader(sw.Workspace, ".offload")
		tk.Use(toolkit.NewOffloadMiddlewareWithOffloader(offloader, "", ".offload"))
		if deps.ToolOffload != nil {
			tk.Use(NewToolOffloadMiddleware(deps.ToolOffload, ""))
		}
		b := react.Builder().
			Name(name).
			SysPrompt(sysPrompt).
			Model(chatModel).
			Workspace(sw.Workspace).
			PermissionEngine(permEngine).
			Hooks(skillHook).
			Toolkit(tk)
		if cfg != nil && cfg.ID != "" {
			b = b.ID(cfg.ID)
		}
		return b.Build()
	}

	offloader := workspace.NewWorkspaceOffloader(sw.Workspace, ".offload")
	tk.Use(toolkit.NewOffloadMiddlewareWithOffloader(offloader, "", ".offload"))
	if deps.ToolOffload != nil {
		tk.Use(NewToolOffloadMiddleware(deps.ToolOffload, ""))
	}

	b := react.Builder().
		Name(name).
		SysPrompt(sysPrompt).
		Model(chatModel).
		Workspace(sw.Workspace).
		PermissionEngine(permEngine).
		Toolkit(tk)
	if cfg != nil && cfg.ID != "" {
		b = b.ID(cfg.ID)
	}
	return b.Build()
}

func (f *AgentFactory) buildModel(cfg *service.AgentConfig, cred *service.Credential) (model.ChatModel, error) {
	if cfg == nil || cred == nil {
		return nil, fmt.Errorf("agent_factory: config and credential required for session agent")
	}
	apiKey, err := f.decryptKey(cred)
	if err != nil {
		return nil, err
	}
	provider, modelName := parseModelID(cfg.ModelID, cred.Provider)
	if provider == "" {
		return nil, fmt.Errorf("agent_factory: cannot determine provider for model %q", cfg.ModelID)
	}
	builder, ok := f.modelBuilders[provider]
	if !ok {
		return nil, fmt.Errorf("agent_factory: unsupported provider %q", provider)
	}
	baseURL := ""
	if cfg.Metadata != nil {
		if u, _ := cfg.Metadata["base_url"].(string); u != "" {
			baseURL = u
		}
	}
	return builder(apiKey, modelName, baseURL)
}

func sessionWorkspaceTools(wsDir string, deps SessionAgentDeps) []tool.Tool {
	_ = os.MkdirAll(wsDir, 0o755)
	tools := []tool.Tool{
		file.NewReadFileTool(wsDir),
		file.NewListDirectoryTool(wsDir),
		file.NewWriteFileTool(wsDir),
		shell.NewShellCommandTool(wsDir, []string{"ls", "cat", "pwd", "echo"}, nil),
	}
	if deps.TaskStore != nil {
		tools = append(tools, tasktool.RegisterTools(deps.TaskStore)...)
	}
	tools = append(tools, deps.ExtraTools...)
	if deps.TaskStop != nil {
		tools = append(tools, deps.TaskStop)
	}
	if deps.ScheduleMgr != nil {
		tools = append(tools, scheduletool.RegisterTools(deps.ScheduleMgr)...)
	}
	return tools
}

func defaultWorkspacePermRules() []permission.Rule {
	return []permission.Rule{
		{Target: "tool_name", Pattern: "read_file", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "list_directory", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "TaskGet", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "TaskList", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "ScheduleList", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "ScheduleView", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "Skill", Decision: permission.DecisionAllow},
		{Target: "tool_name", Pattern: "write_file", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "shell_command", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskCreate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskUpdate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "ScheduleCreate", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "ScheduleStop", Decision: permission.DecisionAsk},
		{Target: "tool_name", Pattern: "TaskStop", Decision: permission.DecisionAsk},
	}
}

func (s *Server) resolveAgentForRequest(r *http.Request, agentID, sessionID string) (agent.Agent, error) {
	if sessionID != "" && s.sessionAgentBuilder != nil {
		userID := service.UserIDFromContext(r.Context())
		if userID == "" && s.storage != nil {
			if se, err := s.storage.GetSession(r.Context(), sessionID); err == nil {
				userID = se.UserID
				if agentID == "" {
					agentID = se.AgentID
				}
			}
		}
		if userID != "" && agentID != "" {
			return s.sessionAgentBuilder(r.Context(), userID, agentID, sessionID)
		}
	}
	if sessionID != "" && s.registry != nil && s.registry.factory != nil && s.storage != nil && s.workspaceMgr != nil && agentID != "" {
		if ag, err := s.buildSessionAgentFromStorage(r.Context(), agentID, sessionID); err == nil {
			return ag, nil
		}
	}
	return s.resolveAgent(r, agentID)
}

func (s *Server) buildSessionAgentFromStorage(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
	cfg, err := s.storage.GetAgentConfig(ctx, agentID)
	if err != nil {
		return nil, err
	}
	se, err := s.storage.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	creds, err := s.storage.ListCredentialsByUser(ctx, se.UserID)
	if err != nil {
		return nil, err
	}
	var matched *service.Credential
	for _, c := range creds {
		if c.Provider == providerFromModelID(cfg.ModelID) {
			matched = c
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("no credential for provider %q", providerFromModelID(cfg.ModelID))
	}
	sw, err := s.workspaceMgr.GetOrCreate(ctx, s.storage, se.UserID, se.AgentID, sessionID)
	if err != nil {
		return nil, err
	}

	deps := SessionAgentDeps{}
	// Merge server-level auto-assembled defaults (from NewApp with AutoStandardTools etc.)
	if s != nil {
		defaults := s.DefaultSessionDeps()
		if len(defaults.ExtraTools) > 0 {
			deps.ExtraTools = defaults.ExtraTools
		}
		if defaults.ToolOffload != nil {
			deps.ToolOffload = defaults.ToolOffload
		}
		if defaults.PermissionMode != "" {
			deps.PermissionMode = defaults.PermissionMode
		}
		if defaults.ScheduleMgr != nil {
			deps.ScheduleMgr = defaults.ScheduleMgr
		}
		if defaults.TaskStore != nil {
			deps.TaskStore = defaults.TaskStore
		}
	}

	return s.registry.factory.BuildSessionAgent(cfg, matched, sw, deps)
}
