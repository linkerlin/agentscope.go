package tool

import (
	"context"
	"testing"
)

func TestFunctionTool(t *testing.T) {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": "number"},
		},
	}
	tool := NewFunctionTool("add_one", "Adds one to x", params, func(ctx context.Context, input map[string]any) (*Response, error) {
		x, _ := input["x"].(float64)
		return NewTextResponse(x + 1), nil
	})

	if tool.Name() != "add_one" {
		t.Errorf("unexpected name: %s", tool.Name())
	}
	if tool.Description() != "Adds one to x" {
		t.Errorf("unexpected description: %s", tool.Description())
	}

	spec := tool.Spec()
	if spec.Name != "add_one" {
		t.Errorf("spec name mismatch: %s", spec.Name)
	}

	result, err := tool.Execute(context.Background(), map[string]any{"x": float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	if result.GetTextContent() != "6" {
		t.Errorf("expected 6, got %v", result.GetTextContent())
	}
}
