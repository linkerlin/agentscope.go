package plan

import "testing"

func TestEnhancedPlanNotebookHint(t *testing.T) {
	nb := NewEnhancedPlanNotebook()
	_, err := nb.CreatePlan("p", "d", "e", []Subtask{{Name: "s1", State: SubtaskTodo}})
	if err != nil {
		t.Fatal(err)
	}
	h := nb.GenerateHint()
	if h == "" {
		t.Fatal("empty hint")
	}
}
