package plan

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// StepStatus represents the current state of a plan step
type StepStatus string

const (
	StatusPending   StepStatus = "pending"
	StatusRunning   StepStatus = "running"
	StatusCompleted StepStatus = "completed"
	StatusFailed    StepStatus = "failed"
	StatusPaused    StepStatus = "paused"
)

// PlanStep is a single task in a plan
type PlanStep struct {
	ID          string
	Description string
	Status      StepStatus
	Result      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Plan is an ordered collection of steps
type Plan struct {
	ID        string
	Name      string
	Steps     []*PlanStep
	CreatedAt time.Time
	UpdatedAt time.Time
	mu        sync.RWMutex
}

// PlanNotebook manages multiple plans
type PlanNotebook struct {
	mu    sync.RWMutex
	plans map[string]*Plan
}

// NewPlanNotebook creates a new PlanNotebook
func NewPlanNotebook() *PlanNotebook {
	return &PlanNotebook{plans: make(map[string]*Plan)}
}

// CreatePlan creates a new plan with the given name
func (nb *PlanNotebook) CreatePlan(name string) *Plan {
	p := &Plan{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	nb.mu.Lock()
	nb.plans[p.ID] = p
	nb.mu.Unlock()
	return p
}

// AddStep adds a step to an existing plan
func (nb *PlanNotebook) AddStep(planID, description string) (*PlanStep, error) {
	nb.mu.RLock()
	p, ok := nb.plans[planID]
	nb.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	step := &PlanStep{
		ID:          uuid.New().String(),
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	p.mu.Lock()
	p.Steps = append(p.Steps, step)
	p.UpdatedAt = time.Now()
	p.mu.Unlock()
	return step, nil
}

// UpdateStep updates the status and result of a step
func (nb *PlanNotebook) UpdateStep(planID, stepID string, status StepStatus, result string) error {
	nb.mu.RLock()
	p, ok := nb.plans[planID]
	nb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.Steps {
		if s.ID == stepID {
			s.Status = status
			s.Result = result
			s.UpdatedAt = time.Now()
			p.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("step not found: %s", stepID)
}

// GetPlan returns a plan by ID
func (nb *PlanNotebook) GetPlan(planID string) (*Plan, error) {
	nb.mu.RLock()
	p, ok := nb.plans[planID]
	nb.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	return p, nil
}

// ListPlans returns all plans
func (nb *PlanNotebook) ListPlans() []*Plan {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	result := make([]*Plan, 0, len(nb.plans))
	for _, p := range nb.plans {
		result = append(result, p)
	}
	return result
}

// AsTool returns a tool.Tool that exposes plan management to agents
func (nb *PlanNotebook) AsTool() tool.Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: create_plan, add_step, update_step, get_plan, list_plans",
			},
			"plan_id":     map[string]any{"type": "string"},
			"plan_name":   map[string]any{"type": "string"},
			"step_id":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"status":      map[string]any{"type": "string"},
			"result":      map[string]any{"type": "string"},
		},
		"required": []string{"action"},
	}

	return tool.NewFunctionTool(
		"plan_notebook",
		"Manage task plans: create plans, add steps, update step status",
		params,
		func(ctx context.Context, input map[string]any) (any, error) {
			action, _ := input["action"].(string)
			switch action {
			case "create_plan":
				name, _ := input["plan_name"].(string)
				p := nb.CreatePlan(name)
				return map[string]any{"plan_id": p.ID, "name": p.Name}, nil

			case "add_step":
				planID, _ := input["plan_id"].(string)
				desc, _ := input["description"].(string)
				step, err := nb.AddStep(planID, desc)
				if err != nil {
					return nil, err
				}
				return map[string]any{"step_id": step.ID, "description": step.Description}, nil

			case "update_step":
				planID, _ := input["plan_id"].(string)
				stepID, _ := input["step_id"].(string)
				status, _ := input["status"].(string)
				result, _ := input["result"].(string)
				return nil, nb.UpdateStep(planID, stepID, StepStatus(status), result)

			case "get_plan":
				planID, _ := input["plan_id"].(string)
				p, err := nb.GetPlan(planID)
				if err != nil {
					return nil, err
				}
				return formatPlan(p), nil

			case "list_plans":
				plans := nb.ListPlans()
				result := make([]map[string]any, 0, len(plans))
				for _, p := range plans {
					result = append(result, map[string]any{"plan_id": p.ID, "name": p.Name, "step_count": len(p.Steps)})
				}
				return result, nil

			default:
				return nil, errors.New("unknown action: " + action)
			}
		},
	)
}

func formatPlan(p *Plan) map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	steps := make([]map[string]any, 0, len(p.Steps))
	for _, s := range p.Steps {
		steps = append(steps, map[string]any{
			"step_id":     s.ID,
			"description": s.Description,
			"status":      string(s.Status),
			"result":      s.Result,
		})
	}
	return map[string]any{"plan_id": p.ID, "name": p.Name, "steps": steps}
}

// Ensure PlanNotebook's tool satisfies model.ToolSpec
var _ model.ToolSpec = model.ToolSpec{}
