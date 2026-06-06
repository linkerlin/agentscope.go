package toolkit

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

type stubTool struct{ name string }

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return "stub" }
func (s *stubTool) Spec() model.ToolSpec {
	return model.ToolSpec{Name: s.name, Description: "stub"}
}
func (s *stubTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	return tool.NewTextResponse("ok"), nil
}

func TestResetToolsMetaTool(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&stubTool{name: "t1"})
	_ = reg.Register(&stubTool{name: "t2"})

	gm := NewGroupManager(reg)
	_ = gm.CreateGroup("g1", "group 1")
	_ = gm.AddTool("g1", "t1")
	_ = gm.CreateGroup("g2", "group 2")
	_ = gm.AddTool("g2", "t2")

	meta := NewResetToolsMetaTool(gm)

	ctx := context.Background()

	// Activate g1 only
	resp, err := meta.Execute(ctx, map[string]any{"groups": []any{"g1"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatal("expected response")
	}

	active := gm.ActiveTools()
	if len(active) != 1 || active[0].Name() != "t1" {
		t.Fatalf("expected only t1 active, got %v", active)
	}

	// Activate g2 only
	_, _ = meta.Execute(ctx, map[string]any{"groups": []any{"g2"}})
	active = gm.ActiveTools()
	if len(active) != 1 || active[0].Name() != "t2" {
		t.Fatalf("expected only t2 active, got %v", active)
	}

	// Deactivate all
	_, _ = meta.Execute(ctx, map[string]any{"groups": []any{}})
	active = gm.ActiveTools()
	if len(active) != 2 {
		t.Fatalf("expected all tools active when no groups, got %d", len(active))
	}
}

func TestResetToolsMetaTool_UnknownGroup(t *testing.T) {
	reg := NewRegistry()
	gm := NewGroupManager(reg)
	meta := NewResetToolsMetaTool(gm)

	resp, err := meta.Execute(context.Background(), map[string]any{"groups": []any{"unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if text == "" {
		t.Fatal("expected non-empty response")
	}
}
