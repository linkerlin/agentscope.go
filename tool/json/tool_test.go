package jsontool

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/tool"
)

func TestParseTool_ValidObject(t *testing.T) {
	p := NewParseTool()
	resp, err := p.Execute(context.Background(), map[string]any{
		"json_string": `{"name":"Alice","age":30}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "Alice") {
		t.Fatalf("expected Alice in output, got: %s", text)
	}
	if !strings.Contains(text, "Type: object") {
		t.Fatalf("expected type info, got: %s", text)
	}
}

func TestParseTool_ValidArray(t *testing.T) {
	p := NewParseTool()
	resp, err := p.Execute(context.Background(), map[string]any{
		"json_string": `[1,2,3]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Type: array") {
		t.Fatalf("expected array type, got: %s", resp.GetTextContent())
	}
}

func TestParseTool_InvalidJSON(t *testing.T) {
	p := NewParseTool()
	resp, err := p.Execute(context.Background(), map[string]any{
		"json_string": "{not valid}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "JSONParseError") {
		t.Fatalf("expected error, got: %s", resp.GetTextContent())
	}
}

func TestParseTool_EmptyInput(t *testing.T) {
	p := NewParseTool()
	resp, err := p.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "JSONParseError") {
		t.Fatalf("expected error for empty input, got: %s", resp.GetTextContent())
	}
}

func TestParseTool_StringAndNumber(t *testing.T) {
	tests := []struct {
		name string
		json string
		typ  string
	}{
		{"string", `"hello"`, "string"},
		{"number", `42`, "number"},
		{"boolean", `true`, "boolean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParseTool()
			resp, _ := p.Execute(context.Background(), map[string]any{"json_string": tt.json})
			if !strings.Contains(resp.GetTextContent(), "Type: "+tt.typ) {
				t.Fatalf("expected %s type, got: %s", tt.typ, resp.GetTextContent())
			}
		})
	}
}

func TestQueryTool_NestedObject(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{
		"json_string": `{"user":{"name":"Alice","address":{"city":"NYC"}}}`,
		"path":        "user.address.city",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "NYC") {
		t.Fatalf("expected NYC, got: %s", text)
	}
}

func TestQueryTool_ArrayIndex(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{
		"json_string": `{"items":[{"name":"A"},{"name":"B"}]}`,
		"path":        "items.1.name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "B") {
		t.Fatalf("expected B, got: %s", resp.GetTextContent())
	}
}

func TestQueryTool_KeyNotFound(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{
		"json_string": `{"a":1}`,
		"path":        "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "JSONQueryError") {
		t.Fatalf("expected error, got: %s", resp.GetTextContent())
	}
}

func TestQueryTool_IndexOutOfRange(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{
		"json_string": `[1,2]`,
		"path":        "5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "JSONQueryError") {
		t.Fatalf("expected error, got: %s", resp.GetTextContent())
	}
}

func TestQueryTool_MissingInput(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "JSONQueryError") {
		t.Fatalf("expected error, got: %s", resp.GetTextContent())
	}
}

func TestQueryTool_MissingPath(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{
		"json_string": `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "JSONQueryError") {
		t.Fatalf("expected error, got: %s", resp.GetTextContent())
	}
}

func TestQueryTool_TopLevelValue(t *testing.T) {
	q := NewQueryTool()
	resp, err := q.Execute(context.Background(), map[string]any{
		"json_string": `"top-level-string"`,
		"path":        "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Index into a string should fail
	if !strings.Contains(resp.GetTextContent(), "JSONQueryError") {
		t.Fatalf("expected error when indexing into string, got: %s", resp.GetTextContent())
	}
}

func TestSpec(t *testing.T) {
	p := NewParseTool()
	if p.Spec().Name != "json_parse" {
		t.Fatal("bad spec name")
	}
	q := NewQueryTool()
	if q.Spec().Name != "json_query" {
		t.Fatal("bad spec name")
	}
}

func TestInterfaceCheck(t *testing.T) {
	var _ tool.Tool = (*ParseTool)(nil)
	var _ tool.Tool = (*QueryTool)(nil)
}

// FuzzParseTool is a basic fuzz test for json_parse tool (P3 testing enhancement example).
// Run with: go test -fuzz=FuzzParseTool -fuzztime=10s ./tool/json
func FuzzParseTool(f *testing.F) {
	p := NewParseTool()
	f.Add(`{"a":1,"b":"x"}`)
	f.Add(`[1,2,3]`)
	f.Add(`"string"`)
	f.Add(`null`)
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = p.Execute(context.Background(), map[string]any{
			"json_string": input,
		})
	})
}
