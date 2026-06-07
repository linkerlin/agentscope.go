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

// FunctionTool wraps a Go function as a Tool
type FunctionTool struct {
	name        string
	description string
	parameters  map[string]any
	fn          func(ctx context.Context, input map[string]any) (*Response, error)
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
