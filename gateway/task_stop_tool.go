package gateway

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// NewTaskStopTool returns a tool that cancels a background offloaded tool task.
func NewTaskStopTool(mgr *ToolOffloadManager) tool.Tool {
	return &taskStopTool{mgr: mgr}
}

type taskStopTool struct {
	mgr *ToolOffloadManager
}

func (t *taskStopTool) Name() string { return "TaskStop" }
func (t *taskStopTool) Description() string {
	return "Cancel a background offloaded tool task by task_id."
}
func (t *taskStopTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string"},
			},
			"required": []string{"task_id"},
		},
	}
}
func (t *taskStopTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	if t.mgr == nil {
		return tool.NewTextResponse("TaskStopError: no offload manager configured"), nil
	}
	taskID, _ := input["task_id"].(string)
	if taskID == "" {
		return tool.NewTextResponse("TaskStopError: task_id is required"), nil
	}
	if !t.mgr.Cancel(taskID) {
		return tool.NewTextResponse(fmt.Sprintf("TaskStopError: task %s not found or already finished", taskID)), nil
	}
	return tool.NewTextResponse(fmt.Sprintf("Background task %s cancelled.", taskID)), nil
}
