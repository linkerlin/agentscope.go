package scheduletool

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/schedule"
	"github.com/linkerlin/agentscope.go/tool"
)

// Manager abstracts cron scheduling for agent-callable schedule tools.
type Manager interface {
	Schedule(ctx context.Context, job *schedule.Job) error
	Cancel(ctx context.Context, jobID string) error
	NextRun(jobID string) (string, error)
	List() []*schedule.Job
}

// RegisterTools returns ScheduleCreate/List/Stop/View tools.
func RegisterTools(mgr Manager) []tool.Tool {
	return []tool.Tool{
		NewCreateTool(mgr),
		NewListTool(mgr),
		NewStopTool(mgr),
		NewViewTool(mgr),
	}
}

type createTool struct{ mgr Manager }

func NewCreateTool(mgr Manager) tool.Tool { return &createTool{mgr: mgr} }

func (t *createTool) Name() string { return "ScheduleCreate" }
func (t *createTool) Description() string {
	return "Create a recurring scheduled task for the current agent using a cron expression."
}
func (t *createTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":         map[string]any{"type": "string", "description": "Unique schedule ID"},
				"agent_id":   map[string]any{"type": "string"},
				"session_id": map[string]any{"type": "string"},
				"cron_expr":  map[string]any{"type": "string", "description": "5-field cron expression"},
				"payload":    map[string]any{"type": "string", "description": "User message sent on each trigger"},
			},
			"required": []string{"id", "agent_id", "cron_expr", "payload"},
		},
	}
}
func (t *createTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	job := &schedule.Job{
		ID:        str(input, "id"),
		AgentID:   str(input, "agent_id"),
		SessionID: str(input, "session_id"),
		CronExpr:  str(input, "cron_expr"),
		Payload:   str(input, "payload"),
		Enabled:   true,
	}
	if job.ID == "" || job.AgentID == "" || job.CronExpr == "" {
		return tool.NewTextResponse("ScheduleCreateError: id, agent_id and cron_expr are required"), nil
	}
	if err := t.mgr.Schedule(ctx, job); err != nil {
		return tool.NewTextResponse("ScheduleCreateError: " + err.Error()), nil
	}
	next, _ := t.mgr.NextRun(job.ID)
	return tool.NewTextResponse(fmt.Sprintf("Schedule %q created. Next run: %s", job.ID, next)), nil
}

type listTool struct{ mgr Manager }

func NewListTool(mgr Manager) tool.Tool { return &listTool{mgr: mgr} }

func (t *listTool) Name() string        { return "ScheduleList" }
func (t *listTool) Description() string { return "List all scheduled jobs." }
func (t *listTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
	}
}
func (t *listTool) Execute(_ context.Context, _ map[string]any) (*tool.Response, error) {
	jobs := t.mgr.List()
	if len(jobs) == 0 {
		return tool.NewTextResponse("No schedules configured."), nil
	}
	var lines []string
	for _, j := range jobs {
		next, _ := t.mgr.NextRun(j.ID)
		lines = append(lines, fmt.Sprintf("%s agent=%s cron=%s next=%s", j.ID, j.AgentID, j.CronExpr, next))
	}
	return tool.NewTextResponse(fmt.Sprintf("%d schedule(s):\n%s", len(jobs), joinLines(lines))), nil
}

type stopTool struct{ mgr Manager }

func NewStopTool(mgr Manager) tool.Tool { return &stopTool{mgr: mgr} }

func (t *stopTool) Name() string        { return "ScheduleStop" }
func (t *stopTool) Description() string { return "Cancel a scheduled job by ID." }
func (t *stopTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
			"required": []string{"id"},
		},
	}
}
func (t *stopTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	id := str(input, "id")
	if err := t.mgr.Cancel(ctx, id); err != nil {
		return tool.NewTextResponse("ScheduleStopError: " + err.Error()), nil
	}
	return tool.NewTextResponse(fmt.Sprintf("Schedule %q stopped.", id)), nil
}

type viewTool struct{ mgr Manager }

func NewViewTool(mgr Manager) tool.Tool { return &viewTool{mgr: mgr} }

func (t *viewTool) Name() string        { return "ScheduleView" }
func (t *viewTool) Description() string { return "View details and next run time for a schedule." }
func (t *viewTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name: t.Name(), Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
			"required": []string{"id"},
		},
	}
}
func (t *viewTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	id := str(input, "id")
	for _, j := range t.mgr.List() {
		if j.ID != id {
			continue
		}
		next, _ := t.mgr.NextRun(id)
		return tool.NewTextResponse(fmt.Sprintf("Schedule %s: agent=%s session=%s cron=%s payload=%q next=%s",
			j.ID, j.AgentID, j.SessionID, j.CronExpr, j.Payload, next)), nil
	}
	return tool.NewTextResponse("Schedule not found: " + id), nil
}

func str(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
