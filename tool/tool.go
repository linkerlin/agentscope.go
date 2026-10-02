package tool

import (
	"context"

	"github.com/linkerlin/agentscope.go/model"
)

// Tool is the interface all tools must implement
type Tool interface {
	Name() string
	Description() string
	Spec() model.ToolSpec
	Execute(ctx context.Context, input map[string]any) (*Response, error)
}

// ChunkedTool is a Tool whose execution emits incremental output chunks
// (20.6): long-running commands streaming stdout, progress lines, staged
// results. emit is called synchronously from the executing goroutine —
// chunk order is emit order, and the caller's stream preserves it. The
// final Response remains the authoritative result (chunks are live output,
// not the result); a cancellation (ctx) surfaces as the returned error
// while chunks already emitted stay delivered.
type ChunkedTool interface {
	Tool
	ExecuteChunked(ctx context.Context, input map[string]any, emit func(chunk string)) (*Response, error)
}

// ReadOnlyChecker is an optional interface tools may implement to declare
// whether they are read-only operations. The permission engine uses this
// information for EXPLORE and ACCEPT_EDITS mode logic.
type ReadOnlyChecker interface {
	Tool
	IsReadOnly() bool
}

// ExternalChecker marks tools that must be executed by an external client.
type ExternalChecker interface {
	Tool
	IsExternalTool() bool
}

// AutoApprovedChecker marks tools that never require a permission prompt
// because the interaction itself is the consent (e.g. AskUser: "never asks
// to ask"). Deny rules still take priority over auto approval.
type AutoApprovedChecker interface {
	Tool
	IsAutoApproved() bool
}

// MCPChecker marks tools backed by an MCP server (PyV2 MCPTool).
type MCPChecker interface {
	Tool
	IsMCPTool() bool
	MCPName() string
}

// FunctionTool wraps a Go function as a Tool
type FunctionTool struct {
	name            string
	description     string
	parameters      map[string]any
	fn              func(ctx context.Context, input map[string]any) (*Response, error)
	readOnly        bool
	concurrencySafe bool
}

// NewFunctionTool creates a Tool from a Go function that returns *Response.
func NewFunctionTool(
	name, description string,
	parameters map[string]any,
	fn func(ctx context.Context, input map[string]any) (*Response, error),
) *FunctionTool {
	return &FunctionTool{
		name:        name,
		description: description,
		parameters:  parameters,
		fn:          fn,
	}
}

func (f *FunctionTool) Name() string        { return f.name }
func (f *FunctionTool) Description() string { return f.description }

func (f *FunctionTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        f.name,
		Description: f.description,
		Parameters:  f.parameters,
	}
}

func (f *FunctionTool) Execute(ctx context.Context, input map[string]any) (*Response, error) {
	return f.fn(ctx, input)
}

func (f *FunctionTool) IsReadOnly() bool { return f.readOnly }
