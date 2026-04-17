package plan

import (
	"os"
	"testing"
)

func TestInMemoryStorage(t *testing.T) {
	s := NewInMemoryStorage()
	nb := NewPlanNotebookWithStorage(s)
	p := nb.CreatePlan("test")

	got, err := s.GetPlan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test" {
		t.Fatalf("expected name test, got %s", got.Name)
	}

	plans, err := s.ListPlans()
	if err != nil || len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	if err := s.DeletePlan(p.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetPlan(p.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestJSONFileStorage(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	nb := NewPlanNotebookWithStorage(s)
	p := nb.CreatePlan("json-test")
	step, _ := nb.AddStep(p.ID, "step1")
	nb.UpdateStep(p.ID, step.ID, StatusCompleted, "done")

	// reload from storage via new instance
	s2, _ := NewJSONFileStorage(dir)
	got, err := s2.GetPlan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "json-test" {
		t.Fatalf("expected name json-test, got %s", got.Name)
	}
	if len(got.Steps) != 1 || got.Steps[0].Status != StatusCompleted {
		t.Fatalf("expected persisted step, got %+v", got.Steps)
	}

	plans, err := s2.ListPlans()
	if err != nil || len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	if err := s2.DeletePlan(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.GetPlan(p.ID); err == nil {
		t.Fatal("expected not found after delete")
	}

	// ensure file removed
	path := dir + string(os.PathSeparator) + p.ID + ".json"
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected json file to be removed")
	}
}

func TestJSONFileStorage_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONFileStorage(dir)
	_, err := s.GetPlan("missing")
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestPlanNotebook_StoragePersistOnUpdate(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewJSONFileStorage(dir)
	nb := NewPlanNotebookWithStorage(s)
	p := nb.CreatePlan("persist")
	step, _ := nb.AddStep(p.ID, "s1")

	// verify via fresh storage
	s2, _ := NewJSONFileStorage(dir)
	got, _ := s2.GetPlan(p.ID)
	if len(got.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got.Steps))
	}

	nb.UpdateStep(p.ID, step.ID, StatusRunning, "half")
	s3, _ := NewJSONFileStorage(dir)
	got3, _ := s3.GetPlan(p.ID)
	if got3.Steps[0].Status != StatusRunning {
		t.Fatalf("expected running status, got %s", got3.Steps[0].Status)
	}
}
