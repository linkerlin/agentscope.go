package plan

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTopologicalSort_NoDeps(t *testing.T) {
	steps := map[string]*Subtask{
		"a": {ID: "a"},
		"b": {ID: "b"},
		"c": {ID: "c"},
	}
	batches, err := topologicalSort(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Fatalf("expected 3 in batch, got %d", len(batches[0]))
	}
}

func TestTopologicalSort_LinearChain(t *testing.T) {
	steps := map[string]*Subtask{
		"a": {ID: "a"},
		"b": {ID: "b", Dependencies: []string{"a"}},
		"c": {ID: "c", Dependencies: []string{"b"}},
	}
	batches, err := topologicalSort(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches for linear chain, got %d", len(batches))
	}
}

func TestTopologicalSort_ParallelBranches(t *testing.T) {
	steps := map[string]*Subtask{
		"a": {ID: "a"},
		"b": {ID: "b", Dependencies: []string{"a"}},
		"c": {ID: "c", Dependencies: []string{"a"}},
		"d": {ID: "d", Dependencies: []string{"b", "c"}},
	}
	batches, err := topologicalSort(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches (a / b+c / d), got %d", len(batches))
	}
	if len(batches[0]) != 1 || batches[0][0] != "a" {
		t.Errorf("batch 0 should be [a], got %v", batches[0])
	}
	if len(batches[1]) != 2 {
		t.Errorf("batch 1 should have 2 items (b,c), got %d", len(batches[1]))
	}
	if len(batches[2]) != 1 || batches[2][0] != "d" {
		t.Errorf("batch 2 should be [d], got %v", batches[2])
	}
}

func TestTopologicalSort_Cycle(t *testing.T) {
	steps := map[string]*Subtask{
		"a": {ID: "a", Dependencies: []string{"c"}},
		"b": {ID: "b", Dependencies: []string{"a"}},
		"c": {ID: "c", Dependencies: []string{"b"}},
	}
	_, err := topologicalSort(steps)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateDAG_Valid(t *testing.T) {
	plan := &RichPlan{
		Subtasks: []Subtask{
			{ID: "a"},
			{ID: "b", Dependencies: []string{"a"}},
		},
	}
	if err := ValidateDAG(plan); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDAG_Cycle(t *testing.T) {
	plan := &RichPlan{
		Subtasks: []Subtask{
			{ID: "a", Dependencies: []string{"b"}},
			{ID: "b", Dependencies: []string{"a"}},
		},
	}
	if err := ValidateDAG(plan); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateDAG_MissingDep(t *testing.T) {
	plan := &RichPlan{
		Subtasks: []Subtask{
			{ID: "a", Dependencies: []string{"nonexistent"}},
		},
	}
	if err := ValidateDAG(plan); err == nil {
		t.Fatal("expected missing dep error")
	}
}

func TestDAGExecutor_SequentialExecution(t *testing.T) {
	var order []string
	var mu sync.Mutex

	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		mu.Lock()
		order = append(order, step.ID)
		mu.Unlock()
		return fmt.Sprintf("done-%s", step.ID), nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "s1", Name: "Step 1"},
			{ID: "s2", Name: "Step 2", Dependencies: []string{"s1"}},
			{ID: "s3", Name: "Step 3", Dependencies: []string{"s2"}},
		},
	}

	e := NewDAGExecutor(exec)
	err := e.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 steps executed, got %d", len(order))
	}
	if order[0] != "s1" || order[1] != "s2" || order[2] != "s3" {
		t.Errorf("expected order s1,s2,s3 got %v", order)
	}
	for _, s := range plan.Subtasks {
		if s.State != SubtaskDone {
			t.Errorf("step %s should be done", s.ID)
		}
	}
	if plan.State != PlanStateDone {
		t.Error("plan should be done")
	}
}

func TestDAGExecutor_ParallelBranches(t *testing.T) {
	var count int32

	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		atomic.AddInt32(&count, 1)
		time.Sleep(50 * time.Millisecond)
		return step.ID + "-result", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "root"},
			{ID: "b1", Dependencies: []string{"root"}},
			{ID: "b2", Dependencies: []string{"root"}},
			{ID: "b3", Dependencies: []string{"root"}},
			{ID: "merge", Dependencies: []string{"b1", "b2", "b3"}},
		},
	}

	e := NewDAGExecutor(exec)
	start := time.Now()
	err := e.Execute(context.Background(), plan)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}

	if atomic.LoadInt32(&count) != 5 {
		t.Fatalf("expected 5 steps, got %d", atomic.LoadInt32(&count))
	}

	if elapsed > 200*time.Millisecond {
		t.Logf("warning: execution took %v (parallel branches should be faster)", elapsed)
	}
}

func TestDAGExecutor_DependencyResultsPassed(t *testing.T) {
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		if len(deps) == 0 {
			return "base-output", nil
		}
		var combined string
		for _, v := range deps {
			combined += v
		}
		return combined + "-processed", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "a"},
			{ID: "b", Dependencies: []string{"a"}},
		},
	}

	e := NewDAGExecutor(exec)
	err := e.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Subtasks[1].Outcome != "base-output-processed" {
		t.Errorf("expected 'base-output-processed', got '%s'", plan.Subtasks[1].Outcome)
	}
}

func TestDAGExecutor_Retry(t *testing.T) {
	var attempts int32

	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			return "", fmt.Errorf("transient error (attempt %d)", n)
		}
		return "success", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "s1"},
		},
	}

	e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 3, Backoff: 10 * time.Millisecond}))
	err := e.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
	if plan.Subtasks[0].Outcome != "success" {
		t.Errorf("expected outcome 'success', got '%s'", plan.Subtasks[0].Outcome)
	}
}

func TestDAGExecutor_Failure_StopOnError(t *testing.T) {
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		if step.ID == "fail" {
			return "", errors.New("intentional failure")
		}
		return "ok", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "fail", State: SubtaskTodo},
			{ID: "never", State: SubtaskTodo, Dependencies: []string{"fail"}},
		},
	}

	e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 0}))
	err := e.Execute(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error")
	}

	if plan.Subtasks[0].State != SubtaskAbandoned {
		t.Errorf("expected failed step to be abandoned, got %s", plan.Subtasks[0].State)
	}
	if plan.Subtasks[1].State != SubtaskTodo {
		t.Errorf("dependent step should remain todo, got %s", plan.Subtasks[1].State)
	}
}

func TestDAGExecutor_Failure_ContinueOnError(t *testing.T) {
	var executed []string
	var mu sync.Mutex

	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		mu.Lock()
		executed = append(executed, step.ID)
		mu.Unlock()
		if step.ID == "fail" {
			return "", errors.New("intentional")
		}
		return "ok", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "ok1"},
			{ID: "fail"},
			{ID: "ok2"},
		},
	}

	e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 0}), WithStopOnError(false))
	err := e.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("expected no error with ContinueOnError, got: %v", err)
	}

	if plan.Subtasks[0].State != SubtaskDone || plan.Subtasks[2].State != SubtaskDone {
		t.Error("independent steps should complete")
	}
	if plan.Subtasks[1].State != SubtaskAbandoned {
		t.Error("failed step should be abandoned")
	}
}

func TestDAGExecutor_Callbacks(t *testing.T) {
	var startCount, doneCount, failCount int32

	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		if step.ID == "fail" {
			return "", errors.New("nope")
		}
		return "ok", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "ok"},
			{ID: "fail"},
		},
	}

	e := NewDAGExecutor(exec,
		WithRetry(RetryPolicy{MaxRetries: 0}),
		WithStopOnError(false),
		WithOnStepStart(func(s *Subtask) { atomic.AddInt32(&startCount, 1) }),
		WithOnStepDone(func(s *Subtask, o string) { atomic.AddInt32(&doneCount, 1) }),
		WithOnStepFail(func(s *Subtask, err error) { atomic.AddInt32(&failCount, 1) }),
	)
	_ = e.Execute(context.Background(), plan)

	if atomic.LoadInt32(&startCount) != 2 {
		t.Errorf("expected 2 starts, got %d", atomic.LoadInt32(&startCount))
	}
	if atomic.LoadInt32(&doneCount) != 1 {
		t.Errorf("expected 1 done, got %d", atomic.LoadInt32(&doneCount))
	}
	if atomic.LoadInt32(&failCount) != 1 {
		t.Errorf("expected 1 fail, got %d", atomic.LoadInt32(&failCount))
	}
}

func TestDAGExecutor_SkipAlreadyDone(t *testing.T) {
	var executed int32

	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		atomic.AddInt32(&executed, 1)
		return "done", nil
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "a", State: SubtaskDone, Outcome: "already"},
			{ID: "b", Dependencies: []string{"a"}},
		},
	}

	e := NewDAGExecutor(exec)
	err := e.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("expected 1 execution (skipping done), got %d", atomic.LoadInt32(&executed))
	}
}

func TestReadySteps(t *testing.T) {
	plan := &RichPlan{
		Subtasks: []Subtask{
			{ID: "a", State: SubtaskTodo},
			{ID: "b", State: SubtaskDone},
			{ID: "c", State: SubtaskTodo, Dependencies: []string{"b"}},
			{ID: "d", State: SubtaskTodo, Dependencies: []string{"a"}},
		},
	}

	ready := ReadySteps(plan)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready steps (a and c), got %d: %v", len(ready), ready)
	}

	hasA, hasC := false, false
	for _, id := range ready {
		if id == "a" {
			hasA = true
		}
		if id == "c" {
			hasC = true
		}
	}
	if !hasA || !hasC {
		t.Errorf("expected a and c ready, got %v", ready)
	}
}

func TestDAGExecutor_ContextCancellation(t *testing.T) {
	exec := ExecutorFunc(func(ctx context.Context, step *Subtask, deps map[string]string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
			return "ok", nil
		}
	})

	plan := &RichPlan{
		ID:    "test",
		State: PlanStateActive,
		Subtasks: []Subtask{
			{ID: "s1"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	e := NewDAGExecutor(exec, WithRetry(RetryPolicy{MaxRetries: 0}))
	err := e.Execute(ctx, plan)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
