package tasktool

import (
	"context"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
)

// Store abstracts task persistence for builtin task tools.
type Store interface {
	Create(subject, description string, metadata map[string]any) *state.AgentTask
	Get(id string) (*state.AgentTask, bool)
	List() []state.AgentTask
	Update(id string, fn func(*state.AgentTask) bool) (*state.AgentTask, bool)
	AddBlockRelation(blockID, blockedByID string)
}

// RegisterTools returns the four PyV2-aligned task management tools.
func RegisterTools(store Store) []tool.Tool {
	return []tool.Tool{
		NewCreateTool(store),
		NewGetTool(store),
		NewListTool(store),
		NewUpdateTool(store),
	}
}

type createTool struct{ store Store }

func NewCreateTool(store Store) tool.Tool { return &createTool{store: store} }

func (t *createTool) Name() string        { return "TaskCreate" }
func (t *createTool) Description() string { return "Create a structured task for the current agent session." }
func (t *createTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subject":     map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"metadata":    map[string]any{"type": "object"},
			},
			"required": []string{"subject", "description"},
		},
	}
}
func (t *createTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	subject, _ := input["subject"].(string)
	desc, _ := input["description"].(string)
	if subject == "" || desc == "" {
		return tool.NewTextResponse("CreateTaskError: subject and description are required"), nil
	}
	var meta map[string]any
	if m, ok := input["metadata"].(map[string]any); ok {
		meta = m
	}
	task := t.store.Create(subject, desc, meta)
	return tool.NewTextResponse(fmt.Sprintf("Task %s created successfully: %s", task.ID, task.Subject)), nil
}

type getTool struct{ store Store }

func NewGetTool(store Store) tool.Tool { return &getTool{store: store} }

func (t *getTool) Name() string        { return "TaskGet" }
func (t *getTool) Description() string { return "Retrieve a task by its ID." }
func (t *getTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string"},
			},
			"required": []string{"task_id"},
		},
	}
}
func (t *getTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	id, _ := input["task_id"].(string)
	task, ok := t.store.Get(id)
	if !ok {
		return tool.NewTextResponse("Task not found"), nil
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("Task #%s: %s", task.ID, task.Subject))
	lines = append(lines, fmt.Sprintf("Status: %s", task.State))
	lines = append(lines, fmt.Sprintf("Description: %s", task.Description))
	if task.Owner != "" {
		lines = append(lines, fmt.Sprintf("Owner: %s", task.Owner))
	}
	if len(task.BlockedBy) > 0 {
		lines = append(lines, fmt.Sprintf("Blocked by: %s", strings.Join(task.BlockedBy, ", ")))
	}
	if len(task.Blocks) > 0 {
		lines = append(lines, fmt.Sprintf("Blocks: %s", strings.Join(task.Blocks, ", ")))
	}
	return tool.NewTextResponse(strings.Join(lines, "\n")), nil
}

type listTool struct{ store Store }

func NewListTool(store Store) tool.Tool { return &listTool{store: store} }

func (t *listTool) Name() string        { return "TaskList" }
func (t *listTool) Description() string { return "List all tasks in the current session." }
func (t *listTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
	}
}
func (t *listTool) Execute(_ context.Context, _ map[string]any) (*tool.Response, error) {
	tasks := t.store.List()
	if len(tasks) == 0 {
		return tool.NewTextResponse("No tasks available."), nil
	}
	var lines []string
	for _, task := range tasks {
		owner, blocked := "", ""
		if task.Owner != "" {
			owner = "(" + task.Owner + ")"
		}
		if len(task.BlockedBy) > 0 {
			blocked = fmt.Sprintf("[blocked by %s]", strings.Join(task.BlockedBy, ", "))
		}
		lines = append(lines, fmt.Sprintf("#%s [%s] %s%s%s", task.ID, task.State, task.Subject, owner, blocked))
	}
	return tool.NewTextResponse(strings.Join(lines, "\n")), nil
}

type updateTool struct{ store Store }

func NewUpdateTool(store Store) tool.Tool { return &updateTool{store: store} }

func (t *updateTool) Name() string { return "TaskUpdate" }
func (t *updateTool) Description() string {
	return "Update a task's status, owner, dependencies, or metadata."
}
func (t *updateTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id":        map[string]any{"type": "string"},
				"subject":        map[string]any{"type": "string"},
				"description":    map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string"},
				"owner":          map[string]any{"type": "string"},
				"add_blocks":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"add_blocked_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"metadata":       map[string]any{"type": "object"},
			},
			"required": []string{"task_id"},
		},
	}
}
func (t *updateTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	id, _ := input["task_id"].(string)
	if status, _ := input["status"].(string); status == "deleted" {
		_, ok := t.store.Update(id, func(*state.AgentTask) bool { return true })
		if !ok {
			return tool.NewTextResponse(fmt.Sprintf("TaskNotFoundError: The task %s does not exist.", id)), nil
		}
		return tool.NewTextResponse(fmt.Sprintf("Task %s has been deleted.", id)), nil
	}
	var updated []string
	_, ok := t.store.Update(id, func(task *state.AgentTask) bool {
		if v, ok := input["subject"].(string); ok && v != "" {
			task.Subject = v
			updated = append(updated, "subject")
		}
		if v, ok := input["description"].(string); ok {
			task.Description = v
			updated = append(updated, "description")
		}
		if v, ok := input["status"].(string); ok && v != "" && v != "deleted" {
			task.State = state.TaskState(v)
			updated = append(updated, "status")
		}
		if v, ok := input["owner"].(string); ok {
			task.Owner = v
			updated = append(updated, "owner")
		}
		if blocks, ok := input["add_blocks"].([]any); ok {
			for _, b := range blocks {
				if bid, ok := b.(string); ok {
					t.store.AddBlockRelation(id, bid)
				}
			}
			updated = append(updated, "add_blocks")
		}
		if blockedBy, ok := input["add_blocked_by"].([]any); ok {
			for _, b := range blockedBy {
				if bid, ok := b.(string); ok {
					t.store.AddBlockRelation(bid, id)
				}
			}
			updated = append(updated, "add_blocked_by")
		}
		if meta, ok := input["metadata"].(map[string]any); ok {
			if task.Metadata == nil {
				task.Metadata = map[string]any{}
			}
			for k, v := range meta {
				if v == nil {
					delete(task.Metadata, k)
				} else {
					task.Metadata[k] = v
				}
			}
			updated = append(updated, "metadata")
		}
		return false
	})
	if !ok {
		return tool.NewTextResponse(fmt.Sprintf("TaskNotFoundError: The task %s does not exist.", id)), nil
	}
	if len(updated) == 0 {
		return tool.NewTextResponse(fmt.Sprintf("No updates were made to task #%s.", id)), nil
	}
	msg := fmt.Sprintf("Update task #%s %s.", id, strings.Join(updated, ", "))
	if status, _ := input["status"].(string); status == "completed" {
		msg += "\n\nTask completed. Call TaskList now to find your next available task."
	}
	return tool.NewTextResponse(msg), nil
}
