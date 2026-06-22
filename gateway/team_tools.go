// Package gateway — team_tools.go implements the four agent-team tools that
// realise Python agentscope's async leader/worker collaboration at runtime:
// TeamCreate, AgentCreate, TeamSay, TeamDelete. Each tool binds to a
// per-session identity (TeamToolContext) and operates directly on Storage +
// TeamBus, mirroring Python's "no intermediate service layer" design where
// only TeamDelete delegates to the session service for cascade cleanup.
package gateway

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
)

// TeamToolContext binds the per-session identity that team tools operate on.
// Mirrors Python agentscope's _TeamToolBase (user_id / session_id / agent_id
// injected at agent-assembly time).
type TeamToolContext struct {
	UserID    string
	AgentID   string
	SessionID string
}

// TeamToolDeps holds shared dependencies for all team tools.
type TeamToolDeps struct {
	Storage service.Storage
	Bus     messagebus.TeamBus
}

// teamToolNames is the set of team tool names, used to always-allow them in
// the permission engine (mirrors Python's check_permissions always ALLOW).
var teamToolNames = []string{"TeamCreate", "AgentCreate", "TeamSay", "TeamDelete"}

// newTeamTools returns the team tool set for the given role ("leader" or
// "worker"). A leader gets all four tools; a worker gets only TeamSay (to
// report back). Returns nil if deps are incomplete (team features disabled).
func newTeamTools(role string, tctx TeamToolContext, deps *TeamToolDeps) []tool.Tool {
	if deps == nil || deps.Storage == nil || deps.Bus == nil {
		return nil
	}
	switch role {
	case "worker":
		return []tool.Tool{&teamSayTool{ctx: tctx, deps: deps, role: "worker"}}
	default: // leader
		return []tool.Tool{
			&teamCreateTool{ctx: tctx, deps: deps},
			&agentCreateTool{ctx: tctx, deps: deps},
			&teamSayTool{ctx: tctx, deps: deps, role: "leader"},
			&teamDeleteTool{ctx: tctx, deps: deps},
		}
	}
}

type teamRecipient struct {
	name      string
	sessionID string
}

// resolveSenderName returns the display name for the caller's agent.
func resolveSenderName(ctx context.Context, storage service.Storage, agentID string) string {
	if cfg, err := storage.GetAgentConfig(ctx, agentID); err == nil && cfg != nil && cfg.Name != "" {
		return cfg.Name
	}
	return agentID
}

// --- TeamCreate ---

type teamCreateTool struct {
	ctx  TeamToolContext
	deps *TeamToolDeps
}

func (t *teamCreateTool) Name() string { return "TeamCreate" }
func (t *teamCreateTool) Description() string {
	return "Create a new agent team led by this session. A session can lead at most one team at a time. After creating it, use AgentCreate to spawn worker agents."
}
func (t *teamCreateTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        "TeamCreate",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "Team name."},
				"description": map[string]any{"type": "string", "description": "What this team does."},
			},
			"required": []string{"name"},
		},
	}
}

func (t *teamCreateTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return tool.NewTextResponse("TeamCreate: 'name' is required."), nil
	}
	desc, _ := input["description"].(string)
	storage := t.deps.Storage
	if existing, err := storage.GetTeamByLeaderSession(ctx, t.ctx.SessionID); err == nil && existing != nil {
		return tool.NewTextResponse(fmt.Sprintf("TeamCreate: this session already leads team %q. Call TeamDelete first.", existing.Name)), nil
	}
	team := &service.Team{
		ID:              generateID("team"),
		UserID:          t.ctx.UserID,
		LeaderSessionID: t.ctx.SessionID,
		Name:            name,
		Description:     desc,
	}
	if err := storage.SaveTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("TeamCreate: %w", err)
	}
	if se, err := storage.GetSession(ctx, t.ctx.SessionID); err == nil && se != nil {
		se.TeamID = team.ID
		_ = storage.SaveSession(ctx, se)
	}
	return tool.NewTextResponse(fmt.Sprintf("Team %q created (id=%s). Use AgentCreate to add workers.", name, team.ID)), nil
}

// --- AgentCreate ---

type agentCreateTool struct {
	ctx  TeamToolContext
	deps *TeamToolDeps
}

func (t *agentCreateTool) Name() string { return "AgentCreate" }
func (t *agentCreateTool) Description() string {
	return "Spawn a new worker agent in the current team. The worker runs in its own session, inherits the leader's model, and starts executing the given prompt immediately."
}
func (t *agentCreateTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        "AgentCreate",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":          map[string]any{"type": "string", "description": "Worker name (unique within the team)."},
				"system_prompt": map[string]any{"type": "string", "description": "System prompt for the worker."},
				"prompt":        map[string]any{"type": "string", "description": "Initial task/message to start the worker."},
			},
			"required": []string{"name", "prompt"},
		},
	}
}

func (t *agentCreateTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return tool.NewTextResponse("AgentCreate: 'name' is required."), nil
	}
	prompt, _ := input["prompt"].(string)
	if prompt == "" {
		return tool.NewTextResponse("AgentCreate: 'prompt' is required."), nil
	}
	sysPrompt, _ := input["system_prompt"].(string)
	storage := t.deps.Storage

	team, err := storage.GetTeamByLeaderSession(ctx, t.ctx.SessionID)
	if err != nil || team == nil {
		return tool.NewTextResponse("AgentCreate: this session does not lead any team. Call TeamCreate first."), nil
	}
	for _, m := range team.Members {
		if m.Name == name {
			return tool.NewTextResponse(fmt.Sprintf("AgentCreate: a member named %q already exists in this team.", name)), nil
		}
	}
	leaderCfg, err := storage.GetAgentConfig(ctx, t.ctx.AgentID)
	if err != nil || leaderCfg == nil {
		return nil, fmt.Errorf("AgentCreate: cannot load leader config: %w", err)
	}
	workerAgentID := generateID("agent")
	workerSessionID := generateID("sess")
	workerCfg := &service.AgentConfig{
		ID:           workerAgentID,
		UserID:       leaderCfg.UserID,
		Name:         name,
		SystemPrompt: sysPrompt,
		ModelID:      leaderCfg.ModelID,
		Source:       "team",
	}
	if err := storage.SaveAgentConfig(ctx, workerCfg); err != nil {
		return nil, fmt.Errorf("AgentCreate: save worker config: %w", err)
	}
	workerSession := &service.Session{
		ID:      workerSessionID,
		UserID:  leaderCfg.UserID,
		AgentID: workerAgentID,
		TeamID:  team.ID,
		Source:  "team",
		Title:   name,
	}
	if err := storage.SaveSession(ctx, workerSession); err != nil {
		return nil, fmt.Errorf("AgentCreate: save worker session: %w", err)
	}
	team.Members = append(team.Members, service.TeamMember{
		AgentID:   workerAgentID,
		Name:      name,
		SessionID: workerSessionID,
	})
	if err := storage.SaveTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("AgentCreate: update team: %w", err)
	}
	from := leaderCfg.Name
	if from == "" {
		from = "leader"
	}
	_ = t.deps.Bus.InboxPush(ctx, workerSessionID, messagebus.TeamMessage{From: from, Content: prompt})
	_ = t.deps.Bus.EnqueueWakeup(ctx, workerSessionID)
	return tool.NewTextResponse(fmt.Sprintf("Worker %q created and started (agent=%s, session=%s).", name, workerAgentID, workerSessionID)), nil
}

// --- TeamSay ---

type teamSayTool struct {
	ctx  TeamToolContext
	deps *TeamToolDeps
	role string // "leader" or "worker" (affects description only)
}

func (t *teamSayTool) Name() string { return "TeamSay" }
func (t *teamSayTool) Description() string {
	if t.role == "worker" {
		return "Send a message to teammates (leader or other workers). You MUST report results to the leader when your task completes."
	}
	return "Send a message to team workers, or broadcast to all. Do not poll workers; they report back when done."
}
func (t *teamSayTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        "TeamSay",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{"type": "string", "description": "Message body."},
				"to":      map[string]any{"type": "string", "description": "Recipient name. Omit to broadcast to all teammates."},
			},
			"required": []string{"content"},
		},
	}
}

func (t *teamSayTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	content, _ := input["content"].(string)
	if content == "" {
		return tool.NewTextResponse("TeamSay: 'content' is required."), nil
	}
	to, _ := input["to"].(string)
	storage := t.deps.Storage

	se, err := storage.GetSession(ctx, t.ctx.SessionID)
	if err != nil || se == nil || se.TeamID == "" {
		return tool.NewTextResponse("TeamSay: this session is not in any team."), nil
	}
	team, err := storage.GetTeam(ctx, se.TeamID)
	if err != nil || team == nil {
		return tool.NewTextResponse("TeamSay: team not found."), nil
	}
	from := resolveSenderName(ctx, storage, t.ctx.AgentID)

	// Build recipient directory: leader + all members.
	var recips []teamRecipient
	if leaderSe, err := storage.GetSession(ctx, team.LeaderSessionID); err == nil && leaderSe != nil {
		recips = append(recips, teamRecipient{name: resolveSenderName(ctx, storage, leaderSe.AgentID), sessionID: team.LeaderSessionID})
	}
	for _, m := range team.Members {
		recips = append(recips, teamRecipient{name: m.Name, sessionID: m.SessionID})
	}

	var targets []teamRecipient
	for _, r := range recips {
		if r.sessionID == t.ctx.SessionID {
			continue
		}
		if to == "" || r.name == to {
			targets = append(targets, r)
		}
	}
	if len(targets) == 0 {
		if to == "" {
			return tool.NewTextResponse("TeamSay: no other teammates to deliver to."), nil
		}
		return tool.NewTextResponse(fmt.Sprintf("TeamSay: no teammate named %q.", to)), nil
	}
	for _, r := range targets {
		_ = t.deps.Bus.InboxPush(ctx, r.sessionID, messagebus.TeamMessage{From: from, Content: content})
		_ = t.deps.Bus.EnqueueWakeup(ctx, r.sessionID)
	}
	return tool.NewTextResponse(fmt.Sprintf("Message delivered to %d recipient(s).", len(targets))), nil
}

// --- TeamDelete ---

type teamDeleteTool struct {
	ctx  TeamToolContext
	deps *TeamToolDeps
}

func (t *teamDeleteTool) Name() string { return "TeamDelete" }
func (t *teamDeleteTool) Description() string {
	return "Delete the team led by this session. All worker agents and their sessions are removed. The leader session survives."
}
func (t *teamDeleteTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        "TeamDelete",
		Description: t.Description(),
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
}

func (t *teamDeleteTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	storage := t.deps.Storage
	team, err := storage.GetTeamByLeaderSession(ctx, t.ctx.SessionID)
	if err != nil || team == nil {
		return tool.NewTextResponse("TeamDelete: this session does not lead any team."), nil
	}
	for _, m := range team.Members {
		_ = storage.DeleteAgentConfig(ctx, m.AgentID)
		_ = storage.DeleteSession(ctx, m.SessionID)
	}
	if se, err := storage.GetSession(ctx, team.LeaderSessionID); err == nil && se != nil {
		se.TeamID = ""
		_ = storage.SaveSession(ctx, se)
	}
	n := len(team.Members)
	_ = storage.DeleteTeam(ctx, team.ID)
	return tool.NewTextResponse(fmt.Sprintf("Team %q deleted (%d workers removed).", team.Name, n)), nil
}
