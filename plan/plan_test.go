package plan

import (
	"context"
	"testing"
)

func TestPlanNotebook(t *testing.T) {
	nb := NewPlanNotebook()

	p := nb.CreatePlan("my plan")
	if p.Name != "my plan" {
		t.Errorf("expected 'my plan', got %s", p.Name)
	}

	step, err := nb.AddStep(p.ID, "do something")
	if err != nil {
		t.Fatal(err)
	}

	if err := nb.UpdateStep(p.ID, step.ID, StatusCompleted, "done"); err != nil {
		t.Fatal(err)
	}

	fetched, err := nb.GetPlan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(fetched.Steps))
	}
	if fetched.Steps[0].Status != StatusCompleted {
		t.Errorf("expected completed, got %s", fetched.Steps[0].Status)
	}

	plans := nb.ListPlans()
	if len(plans) != 1 {
		t.Errorf("expected 1 plan, got %d", len(plans))
	}
}

func TestPlanNotebookAsTool(t *testing.T) {
	nb := NewPlanNotebook()
	tool := nb.AsTool()

	if tool.Name() != "plan_notebook" {
		t.Errorf("unexpected tool name: %s", tool.Name())
	}

	result, err := tool.Execute(context.Background(), map[string]any{
		"action":    "create_plan",
		"plan_name": "test plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	planID, _ := m["plan_id"].(string)
	if planID == "" {
		t.Error("expected plan_id in result")
	}
}
