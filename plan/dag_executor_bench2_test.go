package plan

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkValidateDAG(b *testing.B) {
	plan := &RichPlan{}
	for i := 0; i < 50; i++ {
		s := Subtask{
			ID:    fmt.Sprintf("s%d", i),
			Name:  fmt.Sprintf("Step %d", i),
			State: SubtaskTodo,
		}
		if i > 0 {
			s.Dependencies = []string{fmt.Sprintf("s%d", i-1)}
		}
		plan.Subtasks = append(plan.Subtasks, s)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateDAG(plan)
	}
}

func BenchmarkReadySteps(b *testing.B) {
	plan := &RichPlan{}
	for i := 0; i < 50; i++ {
		s := Subtask{
			ID:    fmt.Sprintf("s%d", i),
			Name:  fmt.Sprintf("Step %d", i),
			State: SubtaskTodo,
		}
		if i > 0 {
			s.Dependencies = []string{fmt.Sprintf("s%d", i-1)}
		}
		plan.Subtasks = append(plan.Subtasks, s)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ReadySteps(plan)
	}
}

func BenchmarkDAGExecutor_LargeFanOut(b *testing.B) {
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		return step.ID, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan := &RichPlan{ID: "bench", State: PlanStateActive}
		plan.Subtasks = append(plan.Subtasks, Subtask{ID: "root", State: SubtaskTodo})
		for j := 0; j < 50; j++ {
			plan.Subtasks = append(plan.Subtasks, Subtask{
				ID:           fmt.Sprintf("fan%d", j),
				State:        SubtaskTodo,
				Dependencies: []string{"root"},
			})
		}
		e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 0}))
		_ = e.Execute(context.Background(), plan)
	}
}
