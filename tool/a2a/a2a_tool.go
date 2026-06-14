package a2atool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// A2ATool wraps an a2a.Client as a tool.Tool, enabling a ReAct agent to
// delegate subtasks to remote agents via the A2A protocol. Results are
// returned as tool.Response and automatically injected into the ReAct loop.
type A2ATool struct {
	name        string
	description string
	client      a2a.Client
	timeout     time.Duration
	streaming   bool
	progressFn  func(delta string)
}

// NewA2ATool creates an A2ATool that delegates tasks to a remote agent.
func NewA2ATool(name, description string, client a2a.Client) *A2ATool {
	return &A2ATool{
		name:        name,
		description: description,
		client:      client,
		timeout:     120 * time.Second,
	}
}

// WithTimeout sets the deadline for the remote A2A call.
func (t *A2ATool) WithTimeout(d time.Duration) *A2ATool {
	t.timeout = d
	return t
}

// WithStreaming enables streaming mode (SendSubscribe), collecting intermediate
// progress updates before returning the final result.
func (t *A2ATool) WithStreaming(b bool) *A2ATool {
	t.streaming = b
	return t
}

// WithProgressFn sets an optional callback invoked for each streaming delta.
func (t *A2ATool) WithProgressFn(fn func(delta string)) *A2ATool {
	t.progressFn = fn
	return t
}

func (t *A2ATool) Name() string { return t.name }

func (t *A2ATool) Description() string {
	mode := "synchronous"
	if t.streaming {
		mode = "streaming"
	}
	return fmt.Sprintf("%s (A2A remote agent, %s mode)", t.description, mode)
}

func (t *A2ATool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.name,
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The subtask to delegate to the remote agent. Include all necessary context.",
				},
				"session_id": map[string]any{
					"type":        "string",
					"description": "Optional session ID for conversation continuity with the remote agent.",
				},
			},
			"required": []string{"task"},
		},
	}
}

// Execute sends the task to the remote agent via A2A protocol.
// In streaming mode, progress updates are forwarded to progressFn.
func (t *A2ATool) Execute(ctx context.Context, input map[string]any) (*tool.Response, error) {
	task, _ := input["task"].(string)
	if task == "" {
		return tool.NewErrorResponse(fmt.Errorf("task is required")), nil
	}

	sessionID, _ := input["session_id"].(string)

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	msg := &a2a.Message{
		Role:    "user",
		Content: task,
	}
	if sessionID != "" {
		if msg.Meta == nil {
			msg.Meta = make(map[string]any)
		}
		msg.Meta["session_id"] = sessionID
	}

	if t.streaming {
		return t.executeStreaming(ctx, msg)
	}
	return t.executeSync(ctx, msg)
}

func (t *A2ATool) executeSync(ctx context.Context, msg *a2a.Message) (*tool.Response, error) {
	resp, err := t.client.Send(ctx, msg)
	if err != nil {
		return tool.NewErrorResponse(fmt.Errorf("A2A %s failed: %w", t.name, err)), nil
	}
	if resp == nil || resp.Content == "" {
		return tool.NewErrorResponse(fmt.Errorf("A2A %s returned empty response", t.name)), nil
	}
	return tool.NewTextResponse(resp.Content), nil
}

func (t *A2ATool) executeStreaming(ctx context.Context, msg *a2a.Message) (*tool.Response, error) {
	ch, err := t.client.SendSubscribe(ctx, msg)
	if err != nil {
		return tool.NewErrorResponse(fmt.Errorf("A2A %s stream failed: %w", t.name, err)), nil
	}

	var parts []string
	for {
		select {
		case <-ctx.Done():
			return tool.NewErrorResponse(fmt.Errorf("A2A %s timed out", t.name)), nil
		case m, ok := <-ch:
			if !ok {
				if len(parts) == 0 {
					return tool.NewErrorResponse(fmt.Errorf("A2A %s returned empty stream", t.name)), nil
				}
				return tool.NewTextResponse(strings.Join(parts, "\n")), nil
			}
			if m != nil && m.Content != "" {
				parts = append(parts, m.Content)
				if t.progressFn != nil {
					t.progressFn(m.Content)
				}
			}
		}
	}
}
