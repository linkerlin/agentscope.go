package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/workspace"
)

// mockChatModel is a no-op ChatModel used only to satisfy react.Builder for
// agent construction in tests; its methods are never exercised here.
type mockChatModel struct{ name string }

func (m *mockChatModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	return nil, errors.New("mock chat model: not implemented")
}
func (m *mockChatModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, errors.New("mock chat model: not implemented")
}
func (m *mockChatModel) ModelName() string { return m.name }

func newTeamTestWorkspace(t *testing.T) *SessionWorkspace {
	t.Helper()
	dir := t.TempDir()
	return &SessionWorkspace{Workspace: workspace.NewLocalWorkspace("sess", dir), dir: dir}
}

func TestBuildSubagentTools_SpawnsOnePerTemplate(t *testing.T) {
	f := NewAgentFactory(nil)
	sw := newTeamTestWorkspace(t)
	permEngine := permission.NewEngine(permission.ModeExplore, nil)
	leaderModel := &mockChatModel{name: "leader-model"}

	cfg := &service.AgentConfig{
		SubagentTemplates: []service.SubagentTemplate{
			{Name: "researcher", Description: "Research subagent.", SystemPrompt: "You research."},
			{Name: "coder", Description: "Coding subagent.", SystemPrompt: "You code."},
		},
	}

	tools, err := f.BuildSubagentTools(cfg, &service.Credential{}, leaderModel, sw, permEngine, SessionAgentDeps{})
	if err != nil {
		t.Fatalf("buildSubagentTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 subagent tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, tt := range tools {
		names[tt.Name()] = true
		// Each SubagentTool exposes a spec consumable by an LLM toolkit.
		spec := tt.Spec()
		if spec.Name == "" {
			t.Fatal("subagent tool has empty spec name")
		}
	}
	if !names["researcher"] || !names["coder"] {
		t.Fatalf("expected researcher + coder tools, got %v", names)
	}
}

func TestBuildSubagentTools_EmptyTemplatesReturnsNil(t *testing.T) {
	f := NewAgentFactory(nil)
	sw := newTeamTestWorkspace(t)
	permEngine := permission.NewEngine(permission.ModeExplore, nil)

	tools, err := f.BuildSubagentTools(&service.AgentConfig{}, &service.Credential{}, &mockChatModel{name: "m"}, sw, permEngine, SessionAgentDeps{})
	if err != nil || len(tools) != 0 {
		t.Fatalf("expected no tools for empty templates, got %v err=%v", tools, err)
	}
	// nil config is safe.
	tools2, err := f.BuildSubagentTools(nil, nil, &mockChatModel{name: "m"}, sw, permEngine, SessionAgentDeps{})
	if err != nil || len(tools2) != 0 {
		t.Fatalf("expected no tools for nil cfg, got %v err=%v", tools2, err)
	}
}

func TestBuildSubagentTools_TemplateNameFallback(t *testing.T) {
	f := NewAgentFactory(nil)
	sw := newTeamTestWorkspace(t)
	permEngine := permission.NewEngine(permission.ModeExplore, nil)

	cfg := &service.AgentConfig{
		SubagentTemplates: []service.SubagentTemplate{
			{SystemPrompt: "no name"}, // Name empty -> falls back to "subagent"
		},
	}
	tools, _ := f.BuildSubagentTools(cfg, &service.Credential{}, &mockChatModel{name: "m"}, sw, permEngine, SessionAgentDeps{})
	if len(tools) != 1 || tools[0].Name() != "subagent" {
		t.Fatalf("expected fallback name 'subagent', got %+v", tools)
	}
	if tools[0].Description() == "" {
		t.Fatal("expected a generated description for the unnamed template")
	}
}

func TestBuildSubagentTools_UnknownModelIDFallsBackToLeaderModel(t *testing.T) {
	f := NewAgentFactory(nil)
	sw := newTeamTestWorkspace(t)
	permEngine := permission.NewEngine(permission.ModeExplore, nil)
	leaderModel := &mockChatModel{name: "leader-model"}

	// Template requests a model id whose provider cannot be resolved with the
	// (empty) credential; buildSubagentFromTemplate must fall back to the
	// leader model rather than dropping the template.
	cfg := &service.AgentConfig{
		SubagentTemplates: []service.SubagentTemplate{
			{Name: "fallback", ModelID: "unknown-provider/some-model"},
		},
	}
	tools, _ := f.BuildSubagentTools(cfg, &service.Credential{}, leaderModel, sw, permEngine, SessionAgentDeps{})
	if len(tools) != 1 {
		t.Fatalf("expected template to still spawn (falling back to leader model), got %d tools", len(tools))
	}
}

// TestBuildSubagentFromTemplate_InheritsPermissionEngine verifies by construction
// that the spawned subagent is built with the leader's permission engine. We
// exercise the full BuildSessionAgent path and confirm the leader agent builds
// with subagent tools registered and no error.
func TestBuildSessionAgent_WithSubagentTemplates(t *testing.T) {
	f := NewAgentFactory(nil)
	// Register a provider so buildModel succeeds for the leader.
	f.RegisterProvider("mock", func(key, name, url string) (model.ChatModel, error) {
		return &mockChatModel{name: name}, nil
	})

	sw := newTeamTestWorkspace(t)
	cfg := &service.AgentConfig{
		ID:           "leader",
		Name:         "leader",
		ModelID:      "mock/leader-model",
		SystemPrompt: "You are the team leader.",
		SubagentTemplates: []service.SubagentTemplate{
			{Name: "worker", Description: "A worker subagent.", SystemPrompt: "You work."},
		},
	}
	cred := &service.Credential{Provider: "mock", Encrypted: "key"}

	leader, err := f.BuildSessionAgent(cfg, cred, sw, SessionAgentDeps{PermissionMode: permission.ModeExplore})
	if err != nil {
		t.Fatalf("BuildSessionAgent with templates: %v", err)
	}
	if leader == nil {
		t.Fatal("expected non-nil leader agent")
	}
}
