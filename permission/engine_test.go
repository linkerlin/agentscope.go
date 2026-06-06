package permission

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestEngine_ModeBypass(t *testing.T) {
	e := NewEngine(ModeBypass, nil)
	evals, err := e.Evaluate([]*message.ToolUseBlock{{Name: "write_text_file"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(evals) != 1 || evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow, got %s", evals[0].Decision)
	}
}

func TestEngine_ModeExplore_ReadAllowed(t *testing.T) {
	e := NewEngine(ModeExplore, nil)
	evals, err := e.Evaluate([]*message.ToolUseBlock{{Name: "read_file"}})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow for read, got %s", evals[0].Decision)
	}
}

func TestEngine_ModeExplore_WriteAsk(t *testing.T) {
	e := NewEngine(ModeExplore, nil)
	evals, err := e.Evaluate([]*message.ToolUseBlock{{Name: "write_text_file"}})
	if err != nil {
		t.Fatal(err)
	}
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask for write, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleToolNameGlob(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "allow-read", Target: "tool_name", Pattern: "read_*", Decision: DecisionAllow},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "read_file"}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow via rule, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleToolNameRegex(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "deny-delete", Target: "regex", Pattern: `delete|rm\s+-rf`, Decision: DecisionDeny},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command", Input: map[string]any{"command": "rm -rf /"}}})
	if evals[0].Decision != DecisionDeny {
		t.Fatalf("expected deny via regex, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleFilePathGlob(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "allow-tmp", Target: "file_path", Pattern: "/tmp/*", Decision: DecisionAllow},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "write_text_file", Input: map[string]any{"file_path": "/tmp/test.txt"}}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow via file_path rule, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleCommandSubstring(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "allow-ls", Target: "command", Pattern: "ls", Decision: DecisionAllow},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command", Input: map[string]any{"command": "ls -la"}}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow via command rule, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleCommandRegex(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "allow-curl", Target: "command", Pattern: `^curl\s+https?://`, Decision: DecisionAllow},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command", Input: map[string]any{"command": "curl https://example.com"}}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow via regex command rule, got %s", evals[0].Decision)
	}
}

func TestEngine_EmptyToolCalls(t *testing.T) {
	e := NewEngine(ModeExplore, nil)
	evals, err := e.Evaluate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(evals) != 0 {
		t.Fatalf("expected 0 evaluations, got %d", len(evals))
	}
}

func TestMatchGlob(t *testing.T) {
	if !matchGlob("*", "anything") {
		t.Fatal("* should match anything")
	}
	if !matchGlob("", "anything") {
		t.Fatal("empty pattern should match anything")
	}
	if !matchGlob("read_*", "read_file") {
		t.Fatal("read_* should match read_file")
	}
	if matchGlob("write_*", "read_file") {
		t.Fatal("write_* should not match read_file")
	}
}

func TestLooksLikeRegex(t *testing.T) {
	cases := []struct {
		pattern string
		want    bool
	}{
		{`^curl`, true},
		{`a|b`, true},
		{`foo.*bar`, false}, // .* alone is not detected as regex
		{`read_*`, false},
		{`normal`, false},
	}
	for _, c := range cases {
		got := looksLikeRegex(c.pattern)
		if got != c.want {
			t.Fatalf("looksLikeRegex(%q) = %v, want %v", c.pattern, got, c.want)
		}
	}
}

func TestMatchRegex(t *testing.T) {
	if !matchRegex(`^curl\s+`, "curl -v") {
		t.Fatal("should match")
	}
	if matchRegex(`^curl\s+`, "wget") {
		t.Fatal("should not match")
	}
	// Invalid regex falls back to substring.
	if !matchRegex(`[invalid`, `[invalid`) {
		t.Fatal("invalid regex should fall back to substring")
	}
}


func TestEngine_ModeAcceptEdits(t *testing.T) {
	e := NewEngine(ModeAcceptEdits, nil)

	// Edit tools should be allowed
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "write_text_file"}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow for edit tool, got %s", evals[0].Decision)
	}

	// Shell command should ask
	evals, _ = e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command"}})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask for shell, got %s", evals[0].Decision)
	}

	// Read tools should be allowed
	evals, _ = e.Evaluate([]*message.ToolUseBlock{{Name: "read_file"}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow for read, got %s", evals[0].Decision)
	}
}

func TestEngine_ModeDontAsk(t *testing.T) {
	e := NewEngine(ModeDontAsk, nil)
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command"}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow in dont_ask mode, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleDeny(t *testing.T) {
	e := NewEngine(ModeBypass, []Rule{
		{Name: "block-rm", Target: "command", Pattern: "rm -rf", Decision: DecisionDeny},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command", Input: map[string]any{"command": "rm -rf /"}}})
	if evals[0].Decision != DecisionDeny {
		t.Fatalf("expected deny, got %s", evals[0].Decision)
	}
}

func TestEngine_RuleAsk(t *testing.T) {
	e := NewEngine(ModeBypass, []Rule{
		{Name: "ask-curl", Target: "command", Pattern: "curl", Decision: DecisionAsk},
	})
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command", Input: map[string]any{"command": "curl https://example.com"}}})
	if evals[0].Decision != DecisionAsk {
		t.Fatalf("expected ask, got %s", evals[0].Decision)
	}
}

func TestEngine_MultipleToolCalls(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "allow-read", Target: "tool_name", Pattern: "read_*", Decision: DecisionAllow},
		{Name: "ask-write", Target: "tool_name", Pattern: "write_*", Decision: DecisionAsk},
	})

	evals, _ := e.Evaluate([]*message.ToolUseBlock{
		{Name: "read_file"},
		{Name: "write_text_file"},
	})
	if len(evals) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(evals))
	}
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("first should be allow, got %s", evals[0].Decision)
	}
	if evals[1].Decision != DecisionAsk {
		t.Fatalf("second should be ask, got %s", evals[1].Decision)
	}
}

func TestEngine_RulePriority_FirstMatchWins(t *testing.T) {
	e := NewEngine(ModeExplore, []Rule{
		{Name: "allow-all", Target: "tool_name", Pattern: "*", Decision: DecisionAllow},
		{Name: "deny-rm", Target: "tool_name", Pattern: "execute_shell_command", Decision: DecisionDeny},
	})

	// First rule matches everything, so deny never applies
	evals, _ := e.Evaluate([]*message.ToolUseBlock{{Name: "execute_shell_command"}})
	if evals[0].Decision != DecisionAllow {
		t.Fatalf("expected allow from first rule, got %s", evals[0].Decision)
	}
}

func TestMatchGlobOrRegex(t *testing.T) {
	// Glob pattern
	if !matchGlobOrRegex("read_*", "read_file") {
		t.Fatal("glob should match")
	}
	// Substring fallback
	if !matchGlobOrRegex("ls", "ls -la") {
		t.Fatal("substring fallback should match")
	}
	// Regex pattern
	if !matchGlobOrRegex(`^curl\s+`, "curl -v") {
		t.Fatal("regex should match")
	}
}
