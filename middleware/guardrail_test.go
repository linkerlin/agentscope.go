package middleware

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// secretGuard builds a content guard matching the word "SECRET".
func secretGuard() *ContentGuard {
	return &ContentGuard{Patterns: []GuardPattern{
		{Name: "no-secret", Pattern: regexp.MustCompile(`(?i)\bSECRET\b`)},
	}}
}

// newGuardAgent is the middleware Agent stand-in (identity only).
func newGuardAgent() Agent { return nil }

// TestGuardrail_ModelInput is the model-input surface matrix: hit blocks
// visibly, clean passes through, disabled lets even a hit flow.
func TestGuardrail_ModelInput(t *testing.T) {
	mk := func(enabled bool) *GuardrailMiddleware {
		return NewGuardrailMiddleware(GuardrailConfig{Enabled: enabled, Input: secretGuard()})
	}
	user := message.NewMsg().Role(message.RoleUser).TextContent("tell me the SECRET password").Build()
	clean := message.NewMsg().Role(message.RoleUser).TextContent("hello there").Build()

	nextOK := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}

	// Hit: blocked with the sentinel, nothing runs downstream.
	m := mk(true)
	out, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{user}}, nextOK)
	if !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("input hit must block, got %v", err)
	}
	if out != nil {
		t.Fatal("blocked step must not return a message")
	}
	if len(m.Hits()) != 1 || m.Hits()[0].Surface != "input" || m.Hits()[0].Pattern != "no-secret" {
		t.Fatalf("audit wrong: %+v", m.Hits())
	}

	// Pass: clean input flows, next's answer untouched.
	if out, err = m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{clean}}, nextOK); err != nil || out.GetTextContent() != "ok" {
		t.Fatalf("clean input must pass: %v", err)
	}
	if len(m.Hits()) != 1 {
		t.Fatal("pass must not add audit hits")
	}

	// Off: even a hit flows untouched.
	mOff := mk(false)
	if _, err = mOff.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{user}}, nextOK); err != nil {
		t.Fatalf("disabled guardrail must pass everything, got %v", err)
	}
	if len(mOff.Hits()) != 0 {
		t.Fatal("disabled guardrail must not audit")
	}
}

// TestGuardrail_ModelOutput is the output surface: the model's answer is
// checked on the way out (reasoning step and final reply).
func TestGuardrail_ModelOutput(t *testing.T) {
	m := NewGuardrailMiddleware(GuardrailConfig{Enabled: true, Output: secretGuard()})
	leaky := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("the SECRET is 42").Build(), nil
	}
	fine := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("all clear").Build(), nil
	}

	if _, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{}, leaky); !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("output hit must block, got %v", err)
	}
	if out, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{}, fine); err != nil || out.GetTextContent() != "all clear" {
		t.Fatalf("clean output must pass: %v", err)
	}
	// Same policy on the final reply surface.
	if _, err := m.OnReply(context.Background(), newGuardAgent(), &ReplyInput{}, leaky); !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("reply output hit must block, got %v", err)
	}
	if m.Hits()[0].Surface != "output" || m.Hits()[1].Surface != "output" {
		t.Fatalf("audit surfaces: %+v", m.Hits())
	}
}

// TestGuardrail_ToolText is the tool surface: string inputs and result
// text are checked; the tool itself never runs on a hit.
func TestGuardrail_ToolText(t *testing.T) {
	m := NewGuardrailMiddleware(GuardrailConfig{Enabled: true, Tool: secretGuard()})
	ran := false
	next := func(ctx context.Context) (*tool.Response, error) {
		ran = true
		return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock("ok")}}, nil
	}

	// Hit on the tool input string: blocked before execution.
	_, err := m.OnActing(context.Background(), newGuardAgent(), &ActingInput{
		ToolName: "write_file", ToolInput: map[string]any{"content": "password=SECRET"},
	}, next)
	if !errors.Is(err, ErrGuardrailBlocked) || ran {
		t.Fatalf("tool input hit must block before execution (ran=%v): %v", ran, err)
	}

	// Hit on the tool result text: blocked after execution.
	leaky := func(ctx context.Context) (*tool.Response, error) {
		return &tool.Response{Content: []message.ContentBlock{message.NewTextBlock("leaked SECRET")}}, nil
	}
	if _, err = m.OnActing(context.Background(), newGuardAgent(), &ActingInput{
		ToolName: "read_file", ToolInput: map[string]any{"path": "x"},
	}, leaky); !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("tool output hit must block, got %v", err)
	}

	// Pass: clean tool input and output flow.
	ran = false
	if _, err = m.OnActing(context.Background(), newGuardAgent(), &ActingInput{
		ToolName: "write_file", ToolInput: map[string]any{"content": "safe"},
	}, next); err != nil || !ran {
		t.Fatalf("clean tool call must run: ran=%v err=%v", ran, err)
	}
	if len(m.Hits()) != 2 {
		t.Fatalf("audit: %+v", m.Hits())
	}
}

// TestGuardrail_Binary is the binary surface: oversized or off-allowlist
// blocks are blocked in messages and tool results; small allowlisted ones
// pass; disabled passes everything.
func TestGuardrail_Binary(t *testing.T) {
	cfg := func(on bool) GuardrailConfig {
		return GuardrailConfig{
			Enabled: on,
			Binary: &BinaryGuard{
				MaxBytes:   100,
				AllowTypes: []string{"image/png"},
			},
		}
	}
	bigPNG := message.NewImageBlock("", strings.Repeat("A", 400), "image/png")  // ~300 bytes > 100
	okPNG := message.NewImageBlock("", strings.Repeat("A", 80), "image/png")    // ~60 bytes
	offType := message.NewImageBlock("", strings.Repeat("A", 80), "audio/mpeg") // type not allowed

	userMsg := func(b message.ContentBlock) *message.Msg {
		m := message.NewMsg().Role(message.RoleUser).Build()
		m.Content = append(m.Content, b)
		return m
	}
	nextOK := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}

	m := NewGuardrailMiddleware(cfg(true))
	if _, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{userMsg(bigPNG)}}, nextOK); !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("oversized binary must block, got %v", err)
	}
	if _, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{userMsg(offType)}}, nextOK); !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("off-allowlist binary must block, got %v", err)
	}
	if _, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{userMsg(okPNG)}}, nextOK); err != nil {
		t.Fatalf("allowlisted binary must pass, got %v", err)
	}
	// Tool results carry binary too.
	leakyTool := func(ctx context.Context) (*tool.Response, error) {
		return &tool.Response{Content: []message.ContentBlock{bigPNG}}, nil
	}
	if _, err := m.OnActing(context.Background(), newGuardAgent(), &ActingInput{ToolName: "screenshot", ToolInput: map[string]any{}}, leakyTool); !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("tool binary must block, got %v", err)
	}

	off := NewGuardrailMiddleware(cfg(false))
	if _, err := off.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{userMsg(bigPNG)}}, nextOK); err != nil {
		t.Fatalf("disabled binary guard must pass, got %v", err)
	}
}

// TestGuardrail_NoSilentMutation: a hit NEVER mutates content — the
// blocked message is returned as-is to the caller (visible failure), and
// the audit records the pattern with a bounded excerpt.
func TestGuardrail_NoSilentMutation(t *testing.T) {
	m := NewGuardrailMiddleware(GuardrailConfig{Enabled: true, Output: secretGuard()})
	long := "prefix " + strings.Repeat("x", 300) + " SECRET suffix"
	outMsg := message.NewMsg().Role(message.RoleAssistant).TextContent(long).Build()
	next := func(ctx context.Context) (*message.Msg, error) { return outMsg, nil }

	_, err := m.OnReply(context.Background(), newGuardAgent(), &ReplyInput{}, next)
	if !errors.Is(err, ErrGuardrailBlocked) {
		t.Fatalf("expected block, got %v", err)
	}
	// The caller's message object is untouched (no redaction happened).
	if outMsg.GetTextContent() != long {
		t.Fatal("guardrail must not mutate content in place")
	}
	// The audit excerpt is bounded.
	hit := m.Hits()[0]
	if len(hit.Detail) > 124 || !strings.Contains(hit.Detail, "…") {
		t.Fatalf("audit detail must be bounded: %q", hit.Detail)
	}
}

// TestGuardrail_NoRulesDisabled: a config with every surface nil disables
// itself (nothing to enforce — explicit, not a silent empty pass).
func TestGuardrail_NoRulesDisabled(t *testing.T) {
	m := NewGuardrailMiddleware(GuardrailConfig{Enabled: true})
	user := message.NewMsg().Role(message.RoleUser).TextContent("SECRET everywhere").Build()
	next := func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}
	if _, err := m.OnReasoning(context.Background(), newGuardAgent(), &ReasoningInput{Messages: []*message.Msg{user}}, next); err != nil {
		t.Fatalf("rule-free guardrail must pass, got %v", err)
	}
}

// compile-time: the middleware satisfies the three interceptor interfaces.
var (
	_ ReplyInterceptor     = (*GuardrailMiddleware)(nil)
	_ ReasoningInterceptor = (*GuardrailMiddleware)(nil)
	_ ActingInterceptor    = (*GuardrailMiddleware)(nil)
	_                      = model.ChatOption(nil) // keep the model import shape stable
)
