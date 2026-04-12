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
	Execute(ctx context.Context, input map[string]any) (any, error)
}

// FunctionTool wraps a Go function as a Tool
type FunctionTool struct {
	name        string
	description string
	parameters  map[string]any
	fn          func(ctx context.Context, input map[string]any) (any, error)
}

// NewFunctionTool creates a Tool from a Go function
func NewFunctionTool(
	name, description string,
	parameters map[string]any,
	fn func(ctx context.Context, input map[string]any) (any, error),
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

func (f *FunctionTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	return f.fn(ctx, input)
}
