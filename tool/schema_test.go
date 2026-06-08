package tool

import (
	"context"
	"reflect"
	"testing"
)

type addParams struct {
	X float64 `json:"x" desc:"Number to increment"`
}

func TestExtractInputSchema(t *testing.T) {
	schema := ExtractInputSchema[addParams]()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties, got %#v", schema)
	}
	x, ok := props["x"].(map[string]any)
	if !ok || x["type"] != "number" {
		t.Fatalf("unexpected x schema: %#v", props["x"])
	}
	if x["description"] != "Number to increment" {
		t.Fatalf("unexpected description: %v", x["description"])
	}
	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "x" {
		t.Fatalf("unexpected required: %#v", schema["required"])
	}
}

func TestNewFunctionToolAuto(t *testing.T) {
	ft := NewFunctionToolAuto("add_one", "Adds one", func(ctx context.Context, p addParams) (*Response, error) {
		return NewTextResponse(p.X + 1), nil
	})
	spec := ft.Spec()
	if spec.Parameters["type"] != "object" {
		t.Fatalf("unexpected schema: %#v", spec.Parameters)
	}
	resp, err := ft.Execute(t.Context(), map[string]any{"x": float64(4)})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "5" {
		t.Fatalf("expected 5, got %s", resp.GetTextContent())
	}
}

func TestSchemaFromType_OptionalPointer(t *testing.T) {
	type params struct {
		Required string  `json:"required"`
		Optional *string `json:"optional,omitempty"`
	}
	schema := SchemaFromType(reflect.TypeOf(params{}))
	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "required" {
		t.Fatalf("unexpected required: %#v", schema["required"])
	}
}

func TestFunctionTool_PermissionDefaults(t *testing.T) {
	ft := NewFunctionToolAuto("demo", "d", func(ctx context.Context, p addParams) (*Response, error) {
		return NewTextResponse("ok"), nil
	})
	dec, _, _, passthrough := ft.CheckPermissions(nil, nil)
	if dec != PermAsk || passthrough {
		t.Fatalf("expected ask without passthrough, got %v %v", dec, passthrough)
	}
	if !ft.MatchRule("", nil) || !ft.MatchRule("demo", nil) || ft.MatchRule("other", nil) {
		t.Fatal("unexpected match rule behavior")
	}
	if len(ft.GenerateSuggestions(nil)) != 1 {
		t.Fatal("expected one suggestion")
	}
}
