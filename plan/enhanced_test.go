package plan

import (
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/state"
)

func TestEnhancedPlanNotebook_CreatePlan(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	p, err := nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "p" || p.Description != "d" || p.ExpectedOutcome != "e" {
		t.Fatalf("unexpected plan fields: %+v", p)
	}
	if len(p.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(p.Subtasks))
	}
	if p.Subtasks[0].State != SubtaskTodo {
		t.Fatalf("expected default state todo, got %s", p.Subtasks[0].State)
	}
}

func TestEnhancedPlanNotebook_CurrentPlan(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	if nb.CurrentPlan() != nil {
		t.Fatal("expected nil current plan")
	}
	p, _ := nb.CreatePlan("p", "d", "e", nil)
	if nb.CurrentPlan().ID != p.ID {
		t.Fatal("expected current plan to match created plan")
	}
}

func TestEnhancedPlanNotebook_UpdateSubtaskState(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1"}, {Name: "s2"}})
	if err := nb.UpdateSubtaskState(0, SubtaskInProgress); err != nil {
		t.Fatal(err)
	}
	if nb.CurrentPlan().Subtasks[0].State != SubtaskInProgress {
		t.Fatalf("expected in_progress, got %s", nb.CurrentPlan().Subtasks[0].State)
	}
}

func TestEnhancedPlanNotebook_UpdateSubtaskState_Errors(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	if err := nb.UpdateSubtaskState(0, SubtaskDone); err == nil {
		t.Fatal("expected error with no current plan")
	}
	nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1"}})
	if err := nb.UpdateSubtaskState(-1, SubtaskDone); err == nil {
		t.Fatal("expected error for invalid index")
	}
	if err := nb.UpdateSubtaskState(5, SubtaskDone); err == nil {
		t.Fatal("expected error for out of range index")
	}
}

func TestEnhancedPlanNotebook_FinishSubtask(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1"}})
	if err := nb.FinishSubtask(0, "completed"); err != nil {
		t.Fatal(err)
	}
	st := nb.CurrentPlan().Subtasks[0]
	if st.State != SubtaskDone || st.Outcome != "completed" {
		t.Fatalf("unexpected subtask: %+v", st)
	}
}

func TestEnhancedPlanNotebook_FinishSubtask_Errors(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	if err := nb.FinishSubtask(0, "x"); err == nil {
		t.Fatal("expected error with no current plan")
	}
	nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1"}})
	if err := nb.FinishSubtask(5, "x"); err == nil {
		t.Fatal("expected error for out of range index")
	}
}

func TestEnhancedPlanNotebook_FinishPlan(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	if err := nb.FinishPlan(PlanStateDone, "summary"); err == nil {
		t.Fatal("expected error with no current plan")
	}
	nb.CreatePlan("p", "d", "e", nil)
	if err := nb.FinishPlan(PlanStateDone, "summary"); err != nil {
		t.Fatal(err)
	}
	if nb.CurrentPlan().State != PlanStateDone {
		t.Fatalf("expected done state, got %s", nb.CurrentPlan().State)
	}
}

func TestEnhancedPlanNotebook_ViewHistoricalPlans(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	ids := nb.ViewHistoricalPlans()
	if len(ids) != 0 {
		t.Fatalf("expected 0 historical plans, got %d", len(ids))
	}
	p1, _ := nb.CreatePlan("p1", "d", "e", nil)
	p2, _ := nb.CreatePlan("p2", "d", "e", nil)
	ids = nb.ViewHistoricalPlans()
	if len(ids) != 2 {
		t.Fatalf("expected 2 historical plans, got %d", len(ids))
	}
	if ids[0] != p1.ID || ids[1] != p2.ID {
		t.Fatalf("unexpected historical ids: %v", ids)
	}
}

func TestEnhancedPlanNotebook_RecoverHistoricalPlan(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	p1, _ := nb.CreatePlan("p1", "d", "e", nil)
	nb.CreatePlan("p2", "d", "e", nil)
	if nb.CurrentPlan().Name != "p2" {
		t.Fatal("expected current plan to be p2")
	}
	if err := nb.RecoverHistoricalPlan(p1.ID); err != nil {
		t.Fatal(err)
	}
	if nb.CurrentPlan().Name != "p1" {
		t.Fatal("expected recovered plan p1")
	}
	if err := nb.RecoverHistoricalPlan("missing"); err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestEnhancedPlanNotebook_GenerateHint(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	h := nb.GenerateHint()
	if h != "" {
		t.Fatalf("expected empty hint without plan, got %q", h)
	}
	nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1", State: SubtaskTodo}})
	h = nb.GenerateHint()
	if !strings.Contains(h, "p") || !strings.Contains(h, "s1") {
		t.Fatalf("expected hint to contain plan and subtask names, got %q", h)
	}
}

func TestEnhancedPlanNotebook_SetHintFunc(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	nb.SetHintFunc(func(p *RichPlan) string { return "custom" })
	nb.CreatePlan("p", "d", "e", nil)
	if h := nb.GenerateHint(); h != "custom" {
		t.Fatalf("expected custom hint, got %q", h)
	}
}

func TestEnhancedPlanNotebook_WithStore_CreateAndReload(t *testing.T) {
	dir := t.TempDir()
	store, err := state.NewJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	nb := NewEnhancedPlanNotebookWithStore(store)
	p, _ := nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1"}})
	_ = nb.UpdateSubtaskState(0, SubtaskInProgress)
	_ = nb.FinishSubtask(0, "done")
	_ = nb.FinishPlan(PlanStateDone, "summary")

	// create new notebook with same store
	nb2 := NewEnhancedPlanNotebookWithStore(store)
	cur := nb2.CurrentPlan()
	if cur == nil {
		t.Fatal("expected current plan after reload")
	}
	if cur.Name != "p" || cur.State != PlanStateDone {
		t.Fatalf("unexpected reloaded plan: %+v", cur)
	}
	if len(cur.Subtasks) != 1 || cur.Subtasks[0].State != SubtaskDone {
		t.Fatalf("unexpected reloaded subtask: %+v", cur.Subtasks)
	}
	ids := nb2.ViewHistoricalPlans()
	if len(ids) != 1 || ids[0] != p.ID {
		t.Fatalf("unexpected historical plans: %v", ids)
	}
}

func TestEnhancedPlanNotebook_WithStore_RecoverHistoricalPlan(t *testing.T) {
	dir := t.TempDir()
	store, _ := state.NewJSONStore(dir)
	nb := NewEnhancedPlanNotebookWithStore(store)
	p1, _ := nb.CreatePlan("p1", "d", "e", nil)
	_, _ = nb.CreatePlan("p2", "d", "e", nil)

	// reload and recover p1
	nb2 := NewEnhancedPlanNotebookWithStore(store)
	if err := nb2.RecoverHistoricalPlan(p1.ID); err != nil {
		t.Fatal(err)
	}
	if nb2.CurrentPlan().ID != p1.ID {
		t.Fatal("expected recovered p1")
	}

	// verify p2 still in history
	ids := nb2.ViewHistoricalPlans()
	if len(ids) != 2 {
		t.Fatalf("expected 2 historical plans, got %d", len(ids))
	}
}

func TestEnhancedPlanNotebook_WithStore_NoStore(t *testing.T) {
	nb := NewEnhancedPlanNotebookWithStore(nil)
	p, _ := nb.CreatePlan("p", "d", "e", nil)
	if p == nil {
		t.Fatal("expected plan even without store")
	}
	if nb.CurrentPlan() == nil {
		t.Fatal("expected current plan")
	}
}
