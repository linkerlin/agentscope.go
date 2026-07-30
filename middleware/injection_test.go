package middleware

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

func TestInjectionMiddleware_SystemPrompt(t *testing.T) {
	mw := NewInjectionMiddleware()
	out, err := mw.OnSystemPrompt(context.Background(), &hookStubAgent{"a"}, "You are helpful.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Runtime Awareness") {
		t.Fatalf("system prompt should contain runtime instructions, got: %s", out)
	}
	if !strings.HasPrefix(out, "You are helpful.") {
		t.Fatal("original prompt should be preserved")
	}
}

func TestInjectionMiddleware_SystemPromptDisabled(t *testing.T) {
	cfg := DefaultInjectionConfig()
	cfg.InjectRuntimeState = false
	mw := NewInjectionMiddleware().WithConfig(cfg)
	out, err := mw.OnSystemPrompt(context.Background(), &hookStubAgent{"a"}, "You are helpful.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "You are helpful." {
		t.Fatalf("prompt should be unchanged when disabled, got: %s", out)
	}
}

func TestInjectionMiddleware_ReasoningInjectsTime(t *testing.T) {
	mw := NewInjectionMiddleware().WithTimezone("UTC")

	// First call should inject
	hint := mw.buildHint()
	if hint == "" {
		t.Fatal("first call should produce a hint")
	}
	if !strings.Contains(hint, "Current time:") {
		t.Fatalf("hint should contain time, got: %s", hint)
	}
}

func TestInjectionMiddleware_IntervalRespected(t *testing.T) {
	cfg := DefaultInjectionConfig()
	cfg.TimeInterval = 1 * time.Hour
	mw := NewInjectionMiddleware().WithConfig(cfg)

	first := mw.buildHint()
	if first == "" {
		t.Fatal("first call should produce a hint")
	}
	second := mw.buildHint()
	if second != "" {
		t.Fatal("second call within interval should produce empty hint")
	}
}

func TestInjectionMiddleware_ExtraFields(t *testing.T) {
	mw := NewInjectionMiddleware().WithExtraFields(map[string]string{
		"env":     "production",
		"user_id": "12345",
	})
	hint := mw.buildHint()
	if hint == "" {
		t.Fatal("hint should not be empty")
	}
	if !strings.Contains(hint, "<env>production</env>") {
		t.Fatalf("hint should contain extra field env, got: %s", hint)
	}
	if !strings.Contains(hint, "<user_id>12345</user_id>") {
		t.Fatalf("hint should contain extra field user_id, got: %s", hint)
	}
}

func TestInjectionMiddleware_Template(t *testing.T) {
	cfg := DefaultInjectionConfig()
	cfg.Template = "[{runtime_state}]"
	mw := NewInjectionMiddleware().WithConfig(cfg)
	hint := mw.buildHint()
	if !strings.HasPrefix(hint, "[Current time:") {
		t.Fatalf("hint should be wrapped in template, got: %s", hint)
	}
	if !strings.HasSuffix(hint, "]") {
		t.Fatalf("hint should end with ], got: %s", hint)
	}
}

func TestInjectionMiddleware_ReasoningAppendsMessage(t *testing.T) {
	mw := NewInjectionMiddleware()
	agent := &hookStubAgent{"test"}
	input := &ReasoningInput{
		Iteration: 1,
		Messages:  []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()},
	}

	called := false
	final := ReasoningNext(func(ctx context.Context) (*message.Msg, error) {
		called = true
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	})

	_, err := mw.OnReasoning(context.Background(), agent, input, final)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("next handler should be called")
	}
	// Should have original message + injected hint
	if len(input.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(input.Messages))
	}
	last := input.Messages[len(input.Messages)-1]
	hasHint := false
	for _, b := range last.Content {
		if hb, ok := b.(*message.HintBlock); ok && hb.Kind == "runtime_state" {
			hasHint = true
			break
		}
	}
	if !hasHint {
		t.Fatal("last message should contain runtime_state HintBlock")
	}
}

func TestInjectionMiddleware_TimezoneShanghai(t *testing.T) {
	mw := NewInjectionMiddleware().WithTimezone("Asia/Shanghai")
	hint := mw.buildHint()
	if hint == "" {
		t.Fatal("hint should not be empty")
	}
	// The hint should contain "Current time:" with a valid timestamp
	if !strings.Contains(hint, "Current time:") {
		t.Fatalf("hint should contain time, got: %s", hint)
	}
}
