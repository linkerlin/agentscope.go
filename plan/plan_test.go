package plan

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
)

func TestPlanNotebook_CreatePlan(t *testing.T) {
	nb := NewPlanNotebook()
	p := nb.CreatePlan("test-plan")
	if p == nil || p.Name != "test-plan" {
		t.Fatal("expected plan")
	}
	if len(p.Steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(p.Steps))
	}
}

func TestPlanNotebook_AddStepAndGetPlan(t *testing.T) {
	nb := NewPlanNotebook()
	p := nb.CreatePlan("test")
	step, err := nb.AddStep(p.ID, "step1")
	if err != nil {
		t.Fatal(err)
	}
	if step.Description != "step1" || step.Status != StatusPending {
		t.Fatalf("unexpected step: %+v", step)
	}

	got, err := nb.GetPlan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got.Steps))
	}
}

func TestPlanNotebook_AddStep_PlanNotFound(t *testing.T) {
	nb := NewPlanNotebook()
	_, err := nb.AddStep("missing", "step")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPlanNotebook_UpdateStep(t *testing.T) {
	nb := NewPlanNotebook()
	p := nb.CreatePlan("test")
	step, _ := nb.AddStep(p.ID, "step1")

	err := nb.UpdateStep(p.ID, step.ID, StatusCompleted, "done")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := nb.GetPlan(p.ID)
	if got.Steps[0].Status != StatusCompleted || got.Steps[0].Result != "done" {
		t.Fatalf("unexpected step after update: %+v", got.Steps[0])
	}
}

func TestPlanNotebook_UpdateStep_NotFound(t *testing.T) {
	nb := NewPlanNotebook()
	p := nb.CreatePlan("test")
	if err := nb.UpdateStep(p.ID, "bad-step", StatusCompleted, "done"); err == nil {
		t.Fatal("expected error for missing step")
	}
	if err := nb.UpdateStep("bad-plan", "bad-step", StatusCompleted, "done"); err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestPlanNotebook_ListPlans(t *testing.T) {
	nb := NewPlanNotebook()
	p1 := nb.CreatePlan("a")
	p2 := nb.CreatePlan("b")
	plans := nb.ListPlans()
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	ids := map[string]bool{}
	for _, p := range plans {
		ids[p.ID] = true
	}
	if !ids[p1.ID] || !ids[p2.ID] {
		t.Fatal("expected both plan IDs in list")
	}
}

func TestPlanNotebook_AsTool(t *testing.T) {
	nb := NewPlanNotebook()
	toolImpl := nb.AsTool()
	if toolImpl.Name() != "plan_notebook" {
		t.Fatalf("expected tool name plan_notebook, got %s", toolImpl.Name())
	}

	ctx := context.Background()

	// create_plan
	resp, err := toolImpl.Execute(ctx, map[string]any{"action": "create_plan", "plan_name": "my-plan"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(resp.GetTextContent()), &m); err != nil {
		t.Fatal(err)
	}
	planID := m["plan_id"].(string)

	// add_step
	resp, err = toolImpl.Execute(ctx, map[string]any{"action": "add_step", "plan_id": planID, "description": "do work"})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(resp.GetTextContent()), &m)
	stepID := m["step_id"].(string)

	// update_step
	_, err = toolImpl.Execute(ctx, map[string]any{
		"action":  "update_step",
		"plan_id": planID,
		"step_id": stepID,
		"status":  "completed",
		"result":  "ok",
	})
	if err != nil {
		t.Fatal(err)
	}

	// get_plan
	resp, err = toolImpl.Execute(ctx, map[string]any{"action": "get_plan", "plan_id": planID})
	if err != nil {
		t.Fatal(err)
	}
	var planMap map[string]any
	if err := json.Unmarshal([]byte(resp.GetTextContent()), &planMap); err != nil || planMap["name"] != "my-plan" {
		t.Fatalf("unexpected get_plan response: %v", resp.GetTextContent())
	}

	// list_plans
	resp, err = toolImpl.Execute(ctx, map[string]any{"action": "list_plans"})
	if err != nil {
		t.Fatal(err)
	}
	var list []any
	if err := json.Unmarshal([]byte(resp.GetTextContent()), &list); err != nil || len(list) != 1 {
		t.Fatalf("expected 1 plan in list, got %v", resp.GetTextContent())
	}

	// unknown action
	_, err = toolImpl.Execute(ctx, map[string]any{"action": "destroy"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestPlanNotebook_AsTool_MissingAction(t *testing.T) {
	nb := NewPlanNotebook()
	toolImpl := nb.AsTool()
	_, err := toolImpl.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestFormatPlan(t *testing.T) {
	nb := NewPlanNotebook()
	p := nb.CreatePlan("format-test")
	step, _ := nb.AddStep(p.ID, "s1")
	nb.UpdateStep(p.ID, step.ID, StatusRunning, "half")

	m := formatPlan(p)
	if m["name"] != "format-test" {
		t.Fatalf("unexpected name: %v", m["name"])
	}
	steps, ok := m["steps"].([]map[string]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("expected 1 step, got %v", m["steps"])
	}
	if steps[0]["status"] != "running" {
		t.Fatalf("expected running status, got %v", steps[0]["status"])
	}
}

var _ tool.Tool = (NewPlanNotebook().AsTool())
