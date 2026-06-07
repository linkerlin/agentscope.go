package state

import "testing"

func TestTaskStore_CRUD(t *testing.T) {
	s := NewTaskStore()
	task := s.Create("Fix bug", "Fix auth bug", nil)
	if task.State != TaskPending {
		t.Fatalf("expected pending, got %s", task.State)
	}
	got, ok := s.Get(task.ID)
	if !ok || got.Subject != "Fix bug" {
		t.Fatalf("get failed")
	}
	_, ok = s.Update(task.ID, func(t *AgentTask) bool {
		t.State = TaskInProgress
		return false
	})
	if !ok {
		t.Fatal("update failed")
	}
	got, _ = s.Get(task.ID)
	if got.State != TaskInProgress {
		t.Fatalf("expected in_progress")
	}
}
