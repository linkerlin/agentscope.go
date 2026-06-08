package slowtool

import (
	"context"
	"time"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// DemoTool simulates a long-running operation for Tool Offload demos.
type DemoTool struct {
	Delay time.Duration
}

// New creates a slow_demo tool with the given delay (default 3s).
func New(delay time.Duration) tool.Tool {
	if delay <= 0 {
		delay = 3 * time.Second
	}
	return &DemoTool{Delay: delay}
}

func (t *DemoTool) Name() string { return "slow_demo" }

func (t *DemoTool) Description() string {
	return "Run a slow background-friendly demo search. Results may be offloaded if execution exceeds the gateway timeout."
}

func (t *DemoTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query"},
			},
			"required": []string{"query"},
		},
	}
}

func (t *DemoTool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	query, _ := input["query"].(string)
	if query == "" {
		query = "default"
	}
	select {
	case <-time.After(t.Delay):
		return tool.NewTextResponse("slow_demo result for: " + query), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
