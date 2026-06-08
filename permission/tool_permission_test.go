package permission

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/skill"
	"github.com/linkerlin/agentscope.go/tool"
)

func TestEngine_ToolResolver_ReadOnlyFunctionTool(t *testing.T) {
	custom := tool.NewFunctionToolAuto("allow_me", "always allow", func(ctx context.Context, p struct {
		X string `json:"x"`
	}) (*tool.Response, error) {
		return tool.NewTextResponse(p.X), nil
	}, tool.WithReadOnly(true))
	e := NewEngine(ModeDefault, nil)
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "allow_me" {
			return custom
		}
		return nil
	})
	evals, err := e.Evaluate([]*message.ToolUseBlock{{Name: "allow_me", Input: map[string]any{"x": "ok"}}})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow, got %s", evals[0].Decision)
	}
}

func TestEngine_ToolResolver_SkillViewer(t *testing.T) {
	reg := skill.NewRegistry()
	reg.Register(&skill.AgentSkill{Name: "demo", Description: "d", SkillContent: "c"})
	viewer := skill.NewSkillViewerTool(reg)
	e := NewEngine(ModeDefault, nil)
	e.SetToolResolver(func(name string) tool.Tool {
		if name == "Skill" {
			return viewer
		}
		return nil
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "Skill", Input: map[string]any{"skill": "demo"}}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow for Skill viewer, got %s", evals[0].Decision)
	}
}

func TestEngine_ToolResolver_CustomFunctionAsk(t *testing.T) {
	custom := tool.NewFunctionToolAuto("custom_fn", "d", func(ctx context.Context, p struct {
		X string `json:"x"`
	}) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	})
	e := NewEngine(ModeExplore, nil)
	e.SetToolResolver(func(name string) tool.Tool {
		return custom
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "custom_fn", Input: map[string]any{"x": "1"}}})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask for custom function tool, got %s", evals[0].Decision)
	}
}
