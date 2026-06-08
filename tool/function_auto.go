package tool

import (
	"context"
)

// FunctionToolOption configures FunctionTool metadata.
type FunctionToolOption func(*FunctionTool)

// WithReadOnly marks the tool as read-only for permission checks.
func WithReadOnly(readOnly bool) FunctionToolOption {
	return func(f *FunctionTool) { f.readOnly = readOnly }
}

// WithConcurrencySafe marks whether the tool is safe for concurrent execution.
func WithConcurrencySafe(safe bool) FunctionToolOption {
	return func(f *FunctionTool) { f.concurrencySafe = safe }
}

// NewFunctionToolAuto registers a typed handler and auto-generates JSON Schema from T.
// Handler signature: func(ctx context.Context, input T) (*Response, error)
func NewFunctionToolAuto[T any](
	name, description string,
	fn func(context.Context, T) (*Response, error),
	opts ...FunctionToolOption,
) *FunctionTool {
	schema := ExtractInputSchema[T]()
	ft := &FunctionTool{
		name:            name,
		description:     description,
		parameters:      schema,
		concurrencySafe: true,
		fn: func(ctx context.Context, input map[string]any) (*Response, error) {
			typed, err := decodeInput[T](input)
			if err != nil {
				return nil, err
			}
			return fn(ctx, typed)
		},
	}
	for _, opt := range opts {
		opt(ft)
	}
	return ft
}

func (f *FunctionTool) IsConcurrencySafe() bool { return f.concurrencySafe }

func (f *FunctionTool) CheckPermissions(_ map[string]any, _ any) (PermissionDecision, string, string, bool) {
	if f.readOnly {
		return PermAllow,
			"This is a read-only function tool. Allowing execution.",
			"read-only function tool",
			false
	}
	return PermAsk,
		"Custom function tools must be explicitly allowed by the user.",
		"function tool default",
		false
}

func (f *FunctionTool) MatchRule(pattern string, _ map[string]any) bool {
	return pattern == "" || pattern == f.name
}

func (f *FunctionTool) GenerateSuggestions(_ map[string]any) []SuggestedRule {
	return []SuggestedRule{{
		Name:     "suggested-tool-level",
		ToolName: f.name,
		Target:   "tool_name",
		Pattern:  f.name,
		Decision: PermAllow,
	}}
}
