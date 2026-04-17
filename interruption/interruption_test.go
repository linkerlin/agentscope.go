package interruption

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func TestNewContextDefaults(t *testing.T) {
	c := NewContext()
	if c.Source != SourceUser {
		t.Fatalf("expected default source USER, got %s", c.Source)
	}
	if c.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if c.UserMessage != nil {
		t.Fatal("expected nil user message")
	}
	if len(c.PendingToolCalls) != 0 {
		t.Fatal("expected empty pending tool calls")
	}
}

func TestContextString(t *testing.T) {
	c := NewContext()
	c.Source = SourceSystem
	c.UserMessage = message.NewMsg().Role(message.RoleUser).TextContent("stop").Build()
	c.PendingToolCalls = []*message.ToolUseBlock{
		message.NewToolUseBlock("id1", "tool", map[string]any{}),
	}
	s := c.String()
	if s == "" {
		t.Fatal("expected non-empty string")
	}
	if !contains(s, "SYSTEM") {
		t.Fatalf("expected SYSTEM in string: %s", s)
	}
	if !contains(s, "present") {
		t.Fatalf("expected 'present' in string: %s", s)
	}
	if !contains(s, "1") {
		t.Fatalf("expected pending count 1 in string: %s", s)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
