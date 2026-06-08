package shell

import "testing"

func TestShellCommandTool_MatchRule(t *testing.T) {
	tool := NewShellCommandTool("", nil, nil)
	if !tool.MatchRule("git:*", map[string]any{"command": "git status"}) {
		t.Fatal("expected prefix match")
	}
}

func TestShellCommandTool_GenerateSuggestions(t *testing.T) {
	tool := NewShellCommandTool("", nil, nil)
	rules := tool.GenerateSuggestions(map[string]any{"command": "git commit -m test"})
	if len(rules) != 1 || rules[0].Pattern != "git commit:*" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}

func TestShellCommandTool_GenerateSuggestions_Compound(t *testing.T) {
	tool := NewShellCommandTool("", nil, nil)
	rules := tool.GenerateSuggestions(map[string]any{"command": "git status && npm run build"})
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d: %+v", len(rules), rules)
	}
}
