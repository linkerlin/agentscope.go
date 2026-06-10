package jsontool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// ParseTool parses a JSON string and returns a formatted representation.
type ParseTool struct{}

func NewParseTool() *ParseTool { return &ParseTool{} }

func (t *ParseTool) Name() string { return "json_parse" }

func (t *ParseTool) Description() string {
	return "Parse a JSON string and return a pretty-printed representation. " +
		"Use this to inspect and understand JSON data structures."
}

func (t *ParseTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"json_string": map[string]any{
					"type":        "string",
					"description": "The JSON string to parse.",
				},
			},
			"required": []string{"json_string"},
		},
	}
}

func (t *ParseTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	raw, _ := input["json_string"].(string)
	if raw == "" {
		return tool.NewTextResponse("JSONParseError: json_string is required"), nil
	}

	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return tool.NewTextResponse(fmt.Sprintf("JSONParseError: %v", err)), nil
	}

	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return tool.NewTextResponse(fmt.Sprintf("JSONParseError: format: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("Parsed successfully.\n\n")
	sb.Write(pretty)

	typeName := "object"
	switch v.(type) {
	case []any:
		typeName = "array"
	case float64, json.Number:
		typeName = "number"
	case bool:
		typeName = "boolean"
	case string:
		typeName = "string"
	}
	sb.WriteString(fmt.Sprintf("\n\nType: %s", typeName))

	return tool.NewTextResponse(sb.String()), nil
}

func (t *ParseTool) IsReadOnly() bool { return true }

func (t *ParseTool) CheckPermissions(_ map[string]any, _ any) (tool.PermissionDecision, string, string, bool) {
	return tool.PermAllow, "JSON parse is read-only.", "json_parse", false
}

func (t *ParseTool) MatchRule(pattern string, _ map[string]any) bool {
	return pattern == ""
}

func (t *ParseTool) GenerateSuggestions(_ map[string]any) []tool.SuggestedRule {
	return []tool.SuggestedRule{{
		Name:     "suggested-readonly",
		ToolName: t.Name(),
		Target:   "tool_name",
		Pattern:  t.Name(),
		Decision: tool.PermAllow,
	}}
}

// QueryTool extracts a value from parsed JSON by a dot-separated key path.
type QueryTool struct{}

func NewQueryTool() *QueryTool { return &QueryTool{} }

func (t *QueryTool) Name() string { return "json_query" }

func (t *QueryTool) Description() string {
	return "Query a value from a JSON object using a dot-separated path. " +
		"For example: 'address.city' extracts the nested city from {address: {city: 'NYC'}}. " +
		"Array indices can be used as: 'items.0.name'."
}

func (t *QueryTool) Spec() model.ToolSpec {
	return model.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"json_string": map[string]any{
					"type":        "string",
					"description": "The JSON string to query.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Dot-separated property path (e.g. 'user.name', 'items.0').",
				},
			},
			"required": []string{"json_string", "path"},
		},
	}
}

func (t *QueryTool) Execute(_ context.Context, input map[string]any) (*tool.Response, error) {
	raw, _ := input["json_string"].(string)
	path, _ := input["path"].(string)
	if raw == "" {
		return tool.NewTextResponse("JSONQueryError: json_string is required"), nil
	}
	if path == "" {
		return tool.NewTextResponse("JSONQueryError: path is required"), nil
	}

	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return tool.NewTextResponse(fmt.Sprintf("JSONQueryError: parse: %v", err)), nil
	}

	result, err := queryJSON(v, path)
	if err != nil {
		return tool.NewTextResponse(fmt.Sprintf("JSONQueryError: %v", err)), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return tool.NewTextResponse(fmt.Sprintf("Path: %s\nValue: %s", path, string(pretty))), nil
}

func queryJSON(v any, path string) (any, error) {
	parts := strings.Split(path, ".")
	current := v
	for _, part := range parts {
		switch cur := current.(type) {
		case map[string]any:
			val, ok := cur[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found at path %q", part, path)
			}
			current = val
		case []any:
			idx := 0
			if _, err := fmt.Sscanf(part, "%d", &idx); err != nil || idx < 0 || idx >= len(cur) {
				return nil, fmt.Errorf("index %q out of range at path %q", part, path)
			}
			current = cur[idx]
		default:
			return nil, fmt.Errorf("cannot index into %T at path %q", current, path)
		}
	}
	return current, nil
}

func (t *QueryTool) IsReadOnly() bool { return true }

func (t *QueryTool) CheckPermissions(_ map[string]any, _ any) (tool.PermissionDecision, string, string, bool) {
	return tool.PermAllow, "JSON query is read-only.", "json_query", false
}

func (t *QueryTool) MatchRule(pattern string, _ map[string]any) bool {
	return pattern == ""
}

func (t *QueryTool) GenerateSuggestions(_ map[string]any) []tool.SuggestedRule {
	return []tool.SuggestedRule{{
		Name:     "suggested-readonly",
		ToolName: t.Name(),
		Target:   "tool_name",
		Pattern:  t.Name(),
		Decision: tool.PermAllow,
	}}
}

var _ tool.Tool = (*ParseTool)(nil)
var _ tool.Tool = (*QueryTool)(nil)
