package subagent

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// depthKey is the context key for tracking subagent nesting depth.
type depthKey struct{}

const defaultMaxDepth = 3

// SubagentTool wraps an Agent as a Tool so that another Agent can invoke it.
// This enables recursive multi-agent workflows (e.g., a planner agent delegating
// to specialist agents).
type SubagentTool struct {
	name        string
	description string
	agent       agent.Agent
	maxDepth    int
}

// NewSubagentTool creates a SubagentTool that delegates tasks to the given agent.
// name is the tool name exposed to the parent agent; description should explain
// what kinds of tasks this subagent handles.
func NewSubagentTool(name, description string, a agent.Agent) *SubagentTool {
	return &SubagentTool{
		name:        name,
		description: description,
		agent:       a,
		maxDepth:    defaultMaxDepth,
	}
}

// WithMaxDepth sets the maximum allowed nesting depth. If a subagent tries to
// invoke another subagent beyond this depth, Execute returns an error.
// Default is 3.
func (s *SubagentTool) WithMaxDepth(depth int) *SubagentTool {
	s.maxDepth = depth
	return s
}

// Name returns the tool name.
func (s *SubagentTool) Name() string { return s.name }

// Description returns the tool description.
func (s *SubagentTool) Description() string {
	return fmt.Sprintf("%s (nested depth limit: %d)", s.description, s.maxDepth)
}

// Spec returns the JSON schema for the tool parameters.
func (s *SubagentTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        s.name,
		Description: s.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The task or question to delegate to the subagent. Be specific and include all necessary context.",
				},
			},
			"required": []string{"task"},
		},
	}
}

// Execute runs the subagent with the given task and returns its response.
// It enforces a nesting depth limit to prevent infinite recursion.
func (s *SubagentTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	task, _ := input["task"].(string)
	if task == "" {
		return tool.NewErrorResponse(fmt.Errorf("task is required")), nil
	}

	// Check nesting depth.
	currentDepth, _ := ctx.Value(depthKey{}).(int)
	if currentDepth >= s.maxDepth {
		return tool.NewErrorResponse(fmt.Errorf(
			"subagent nesting depth limit reached (%d). unable to delegate task: %s",
			s.maxDepth, task,
		)), nil
	}
	ctx = context.WithValue(ctx, depthKey{}, currentDepth+1)

	msg := message.NewMsg().Role(message.RoleUser).TextContent(task).Build()
	resp, err := s.agent.Call(ctx, msg)
	if err != nil {
		return tool.NewErrorResponse(fmt.Errorf("subagent %s failed: %w", s.name, err)), nil
	}

	return tool.NewTextResponse(resp.GetTextContent()), nil
}
