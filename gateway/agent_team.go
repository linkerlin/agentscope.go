package gateway

import (
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit"
)

// defaultSubagentMaxDepth caps recursive subagent delegation (matches the
// SubagentTool default).
const defaultSubagentMaxDepth = 3

// BuildSubagentTools turns a leader agent's SubagentTemplates into SubagentTool
// instances, one per template. Each spawned subagent INHERITS the leader's
// permission engine (aligns with Python agentscope #1815: permission context
// inherited from the team leader). Template build failures are skipped
// best-effort so one bad template never breaks the whole team.
//
// This realises the agent-team custom subagent templates from #1833 at runtime.
func (f *AgentFactory) BuildSubagentTools(
	cfg *service.AgentConfig,
	cred *service.Credential,
	leaderModel model.ChatModel,
	sw *SessionWorkspace,
	permEngine *permission.Engine,
	deps SessionAgentDeps,
) ([]tool.Tool, error) {
	if cfg == nil || len(cfg.SubagentTemplates) == 0 {
		return nil, nil
	}
	var tools []tool.Tool
	for i := range cfg.SubagentTemplates {
		tmpl := cfg.SubagentTemplates[i]
		subAgent, err := f.buildSubagentFromTemplate(tmpl, cred, leaderModel, sw, permEngine, deps)
		if err != nil {
			// Best-effort: skip templates that fail to build (e.g. unknown
			// provider) rather than failing the whole leader build.
			continue
		}
		name := tmpl.Name
		if name == "" {
			name = "subagent"
		}
		desc := tmpl.Description
		if desc == "" {
			desc = "Delegate a sub-task to the " + name + " subagent."
		}
		tools = append(tools, agent.NewSubagentTool(subAgent, name, desc, defaultSubagentMaxDepth))
	}
	return tools, nil
}

// buildSubagentFromTemplate builds a single child agent from a template. The
// child reuses the leader's model by default; if the template specifies a
// ModelID resolvable with the same credential, that model is used instead. The
// child shares the leader's permission engine (#1815) and session workspace,
// and gets a basic file/shell toolkit so it can operate independently.
func (f *AgentFactory) buildSubagentFromTemplate(
	tmpl service.SubagentTemplate,
	cred *service.Credential,
	leaderModel model.ChatModel,
	sw *SessionWorkspace,
	permEngine *permission.Engine,
	deps SessionAgentDeps,
) (agent.Agent, error) {
	subModel := leaderModel
	if tmpl.ModelID != "" {
		subCfg := &service.AgentConfig{ModelID: tmpl.ModelID, Metadata: tmpl.Metadata}
		if m, err := f.buildModel(subCfg, cred); err == nil && m != nil {
			subModel = m
		}
	}

	// Give the subagent a basic workspace toolkit so it can read/write files
	// and run allowed commands inside the shared session workspace. Nested
	// subagent tools are intentionally NOT injected, preventing runaway
	// recursion (maxDepth still guards direct delegation).
	subTk := toolkit.NewToolkit()
	for _, t := range sessionWorkspaceTools(sw.dir, deps) {
		if t == nil {
			continue
		}
		_ = subTk.Register(t)
	}

	name := tmpl.Name
	if name == "" {
		name = "subagent"
	}

	b := react.Builder().
		Name(name).
		SysPrompt(tmpl.SystemPrompt).
		Model(subModel).
		Workspace(sw.Workspace).
		PermissionEngine(permEngine).
		Toolkit(subTk)
	return b.Build()
}
