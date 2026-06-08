package file

import "testing"

func TestReadFileTool_MatchRule(t *testing.T) {
	tool := NewReadFileTool("")
	if !tool.MatchRule("src/**", map[string]any{"file_path": "src/main.go"}) {
		t.Fatal("expected match")
	}
	if tool.MatchRule("src/**", map[string]any{}) {
		t.Fatal("expected no match without path")
	}
}

func TestGlobTool_MatchRule(t *testing.T) {
	tool := NewGlobTool("")
	input := map[string]any{"path": "src", "pattern": "**/*.go"}
	if !tool.MatchRule("src", input) {
		t.Fatal("expected path match")
	}
}

func TestEditFileTool_GenerateSuggestions(t *testing.T) {
	tool := NewEditFileTool("")
	rules := tool.GenerateSuggestions(map[string]any{"file_path": "pkg/a.go"})
	if len(rules) != 1 || rules[0].Pattern != "pkg/**" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}
