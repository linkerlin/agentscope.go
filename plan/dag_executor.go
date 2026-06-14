package plan

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// StepExecutor executes a single subtask and returns its outcome string.
// depResults contains the outcomes of all completed dependency subtasks (by ID).
type StepExecutor interface {
	ExecuteStep(ctx context.Context, step *Subtask, depResults map[string]string) (string, error)
}

// ExecutorFunc adapts a function to the StepExecutor interface.
type ExecutorFunc func(ctx context.Context, step *Subtask, depResults map[string]string) (string, error)

func (f ExecutorFunc) ExecuteStep(ctx context.Context, step *Subtask, depResults map[string]string) (string, error) {
	return f(ctx, step, depResults)
}

// RetryPolicy controls retry behavior for failed steps.
type RetryPolicy struct {
	MaxRetries int
	Backoff    time.Duration
}

// DefaultRetryPolicy retries once with a 1s backoff.
var DefaultRetryPolicy = RetryPolicy{MaxRetries: 1, Backoff: time.Second}

// ExecutorOption configures a DAGExecutor.
type ExecutorOption func(*DAGExecutor)

// WithRetry sets the retry policy for failed steps.
func WithRetry(p RetryPolicy) ExecutorOption {
	return func(e *DAGExecutor) { e.retry = p }
}

// WithStopOnError controls whether the executor stops on the first permanent failure.
// Default is true. When false, remaining independent steps continue to execute.
func WithStopOnError(b bool) ExecutorOption {
	return func(e *DAGExecutor) { e.stopOnError = b }
}

// WithOnStepStart sets a callback invoked before each step starts.
func WithOnStepStart(fn func(step *Subtask)) ExecutorOption {
	return func(e *DAGExecutor) { e.onStepStart = fn }
}

// WithOnStepDone sets a callback invoked after each step completes successfully.
func WithOnStepDone(fn func(step *Subtask, outcome string)) ExecutorOption {
	return func(e *DAGExecutor) { e.onStepDone = fn }
}

// WithOnStepFail sets a callback invoked when a step fails permanently.
func WithOnStepFail(fn func(step *Subtask, err error)) ExecutorOption {
	return func(e *DAGExecutor) { e.onStepFail = fn }
}

// DAGExecutor runs a RichPlan respecting subtask Dependencies.
// Steps with all dependencies satisfied execute in parallel.
type DAGExecutor struct {
	executor    StepExecutor
	retry       RetryPolicy
	onStepStart func(step *Subtask)
	onStepDone  func(step *Subtask, outcome string)
	onStepFail  func(step *Subtask, err error)
	stopOnError bool
}

// NewDAGExecutor creates a DAGExecutor with the given step executor and options.
func NewDAGExecutor(exec StepExecutor, opts ...ExecutorOption) *DAGExecutor {
	e := &DAGExecutor{
		executor:    exec,
		retry:       DefaultRetryPolicy,
		stopOnError: true,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs all subtasks in the plan respecting dependencies.
// It returns an error if the plan has a cycle, a step fails permanently
// (and stopOnError is true), or if the context is cancelled.
func (e *DAGExecutor) Execute(ctx context.Context, plan *RichPlan) error {
	if plan == nil {
		return fmt.Errorf("plan: nil plan")
	}
	if len(plan.Subtasks) == 0 {
		return nil
	}

	steps := make(map[string]*Subtask)
	for i := range plan.Subtasks {
		s := &plan.Subtasks[i]
		if s.ID == "" {
			return fmt.Errorf("plan: subtask at index %d has no ID", i)
		}
		steps[s.ID] = s
	}

	for _, s := range plan.Subtasks {
		for _, dep := range s.Dependencies {
			if _, ok := steps[dep]; !ok {
				return fmt.Errorf("plan: subtask %q depends on unknown subtask %q", s.ID, dep)
			}
		}
	}

	order, err := topologicalSort(steps)
	if err != nil {
		return err
	}

	results := sync.Map{}

	for _, batch := range order {
		if err := ctx.Err(); err != nil {
			return err
		}

		var failedStep *Subtask
		var failedErr error

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(len(batch))

		for _, stepID := range batch {
			step := steps[stepID]
			if step.State == SubtaskDone {
				continue
			}

			g.Go(func() error {
				if e.onStepStart != nil {
					e.onStepStart(step)
				}
				step.State = SubtaskInProgress
				plan.UpdatedAt = time.Now()

				depResults := make(map[string]string)
				for _, dep := range step.Dependencies {
					if v, ok := results.Load(dep); ok {
						depResults[dep] = v.(string)
					}
				}

				outcome, execErr := e.executeWithRetry(gctx, step, depResults)
				if execErr != nil {
					step.State = SubtaskAbandoned
					plan.UpdatedAt = time.Now()
					if e.onStepFail != nil {
						e.onStepFail(step, execErr)
					}
					if e.stopOnError {
						failedStep = step
						failedErr = execErr
						return execErr
					}
					return nil
				}

				step.State = SubtaskDone
				step.Outcome = outcome
				plan.UpdatedAt = time.Now()
				results.Store(step.ID, outcome)

				if e.onStepDone != nil {
					e.onStepDone(step, outcome)
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			if e.stopOnError && failedStep != nil {
				return fmt.Errorf("step %q failed: %w", failedStep.Name, failedErr)
			}
		}

		if e.stopOnError {
			for _, stepID := range batch {
				s := steps[stepID]
				if s.State == SubtaskAbandoned {
					return fmt.Errorf("step %q failed and execution stopped", s.Name)
				}
			}
		}
	}

	allDone := true
	for i := range plan.Subtasks {
		if plan.Subtasks[i].State != SubtaskDone {
			allDone = false
			break
		}
	}
	if allDone {
		plan.State = PlanStateDone
	}

	return nil
}

func (e *DAGExecutor) executeWithRetry(ctx context.Context, step *Subtask, depResults map[string]string) (string, error) {
	maxAttempts := e.retry.MaxRetries + 1
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if attempt > 0 && e.retry.Backoff > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(e.retry.Backoff):
			}
		}
		outcome, err := e.executor.ExecuteStep(ctx, step, depResults)
		if err == nil {
			return outcome, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// topologicalSort returns subtask batches using Kahn's algorithm.
// Each batch contains IDs of subtasks that can execute in parallel.
// Returns an error if the dependency graph contains a cycle.
func topologicalSort(steps map[string]*Subtask) ([][]string, error) {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for id, s := range steps {
		inDegree[id] = len(s.Dependencies)
		for _, dep := range s.Dependencies {
			dependents[dep] = append(dependents[dep], id)
		}
	}

	var batches [][]string

	for {
		var ready []string
		for id, deg := range inDegree {
			if deg == 0 {
				ready = append(ready, id)
				delete(inDegree, id)
			}
		}

		if len(ready) == 0 {
			break
		}

		for _, id := range ready {
			for _, dependent := range dependents[id] {
				inDegree[dependent]--
			}
		}

		batches = append(batches, ready)
	}

	if len(inDegree) > 0 {
		var cyclic []string
		for id := range inDegree {
			cyclic = append(cyclic, id)
		}
		return nil, fmt.Errorf("plan: cycle detected involving subtasks: %s", strings.Join(cyclic, ", "))
	}

	return batches, nil
}

// ValidateDAG checks that the plan's dependency graph is valid (no cycles, all deps exist).
func ValidateDAG(plan *RichPlan) error {
	if plan == nil {
		return fmt.Errorf("plan: nil plan")
	}
	steps := make(map[string]*Subtask)
	for i := range plan.Subtasks {
		s := &plan.Subtasks[i]
		if s.ID == "" {
			return fmt.Errorf("plan: subtask at index %d has no ID", i)
		}
		steps[s.ID] = s
	}
	for _, s := range plan.Subtasks {
		for _, dep := range s.Dependencies {
			if _, ok := steps[dep]; !ok {
				return fmt.Errorf("plan: subtask %q depends on unknown %q", s.ID, dep)
			}
		}
	}
	_, err := topologicalSort(steps)
	return err
}

// ReadySteps returns IDs of subtasks whose dependencies are all satisfied (or have no deps).
func ReadySteps(plan *RichPlan) []string {
	if plan == nil {
		return nil
	}
	done := make(map[string]bool)
	for i := range plan.Subtasks {
		if plan.Subtasks[i].State == SubtaskDone {
			done[plan.Subtasks[i].ID] = true
		}
	}
	var ready []string
	for i := range plan.Subtasks {
		s := &plan.Subtasks[i]
		if s.State == SubtaskDone || s.State == SubtaskInProgress {
			continue
		}
		allDepsDone := true
		for _, dep := range s.Dependencies {
			if !done[dep] {
				allDepsDone = false
				break
			}
		}
		if allDepsDone {
			ready = append(ready, s.ID)
		}
	}
	return ready
}
