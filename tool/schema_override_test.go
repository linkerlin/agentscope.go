package tool

import (
	"context"
	"testing"
)

func TestWithInputSchemaOverrides(t *testing.T) {
	custom := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string", "description": "City name"},
		},
		"required": []string{"city"},
	}
	ft := NewFunctionToolAuto("weather", "Get weather",
		func(ctx context.Context, in struct {
			City string `json:"city"`
		}) (*Response, error) {
			return NewTextResponse("sunny in " + in.City), nil
		},
		WithInputSchema(custom),
	)
	spec := ft.Spec()
	if spec.Name != "weather" {
		t.Fatalf("name: %s", spec.Name)
	}
	props, _ := spec.Parameters["properties"].(map[string]any)
	if _, hasCity := props["city"]; !hasCity {
		t.Fatalf("custom schema not applied: %+v", spec.Parameters)
	}
	if _, hasAuto := props["x"]; hasAuto { // struct field is City only, but the override must win exactly
		t.Fatalf("auto schema leaked: %+v", props)
	}
	// The typed handler still works with the override in place.
	resp, err := ft.Execute(context.Background(), map[string]any{"city": "Beijing"})
	if err != nil || resp.GetTextContent() != "sunny in Beijing" {
		t.Fatalf("execute after override: %v %q", err, resp.GetTextContent())
	}
}
