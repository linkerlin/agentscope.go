package plan

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkDAGExecutor_Sequential(b *testing.B) {
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		return step.ID, nil
	})

	subtasks := make([]Subtask, 20)
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("s%d", i)
		subtasks[i] = Subtask{ID: id, Name: id, State: SubtaskTodo}
		if i > 0 {
			subtasks[i].Dependencies = []string{fmt.Sprintf("s%d", i-1)}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan := &RichPlan{
			ID:       "bench",
			State:    PlanStateActive,
			Subtasks: make([]Subtask, len(subtasks)),
		}
		copy(plan.Subtasks, subtasks)
		e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 0}))
		_ = e.Execute(context.Background(), plan)
	}
}

func BenchmarkDAGExecutor_Parallel(b *testing.B) {
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		return step.ID, nil
	})

	subtasks := []Subtask{
		{ID: "root", State: SubtaskTodo},
	}
	for i := 0; i < 19; i++ {
		id := fmt.Sprintf("branch_%d", i)
		subtasks = append(subtasks, Subtask{ID: id, State: SubtaskTodo, Dependencies: []string{"root"}})
	}
	subtasks = append(subtasks, Subtask{ID: "merge", State: SubtaskTodo, Dependencies: []string{"branch_0"}})
	for i := 1; i < 19; i++ {
		subtasks[len(subtasks)-1].Dependencies = append(subtasks[len(subtasks)-1].Dependencies, fmt.Sprintf("branch_%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan := &RichPlan{
			ID:       "bench",
			State:    PlanStateActive,
			Subtasks: make([]Subtask, len(subtasks)),
		}
		copy(plan.Subtasks, subtasks)
		e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 0}))
		_ = e.Execute(context.Background(), plan)
	}
}

func BenchmarkDAGExecutor_TopologicalSort(b *testing.B) {
	steps := make(map[string]*Subtask)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("n%d", i)
		s := &Subtask{ID: id}
		if i > 0 {
			s.Dependencies = []string{fmt.Sprintf("n%d", i-1)}
		}
		steps[id] = s
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = topologicalSort(steps)
	}
}

func BenchmarkDAGExecutor_WithRetry(b *testing.B) {
	var counter int
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		counter++
		return step.ID, nil
	})

	plan := &RichPlan{
		ID:    "bench",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "a", State: SubtaskTodo},
			{ID: "b", State: SubtaskTodo, Dependencies: []string{"a"}},
		},
	}

	e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 2, Backoff: 1 * time.Millisecond}))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan.Subtasks[0].State = SubtaskTodo
		plan.Subtasks[1].State = SubtaskTodo
		_ = e.Execute(context.Background(), plan)
	}
}
