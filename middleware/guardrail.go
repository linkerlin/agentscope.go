// middleware/guardrail.go — the guardrail middleware (20.7): content
// checks on four surfaces — model input, model output, tool text and
// binary blocks. The design rule from the acceptance: DEFAULT
// CONSERVATIVE (a hit blocks the step with a visible error) and NEVER
// silently mutate content — a hit either fails loudly (the caller sees
// ErrGuardrailBlocked and every hit is audited) or passes untouched.
// Redaction-in-place is deliberately not offered: it would change what
// later auditors see.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/linkerlin/agentscope.go/logging"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/tool"
)

// ErrGuardrailBlocked is returned when a guardrail pattern hits. The step
// (reasoning / tool call / reply) fails visibly; nothing is mutated.
var ErrGuardrailBlocked = errors.New("guardrail: content blocked")

// GuardPattern is one content rule: a named regular expression.
type GuardPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

// ContentGuard is a set of patterns applied to a text surface.
type ContentGuard struct {
	Patterns []GuardPattern
}

// Match returns the first pattern that matches s (nil if clean).
func (g *ContentGuard) Match(s string) *GuardPattern {
	if g == nil || s == "" {
		return nil
	}
	for i := range g.Patterns {
		if g.Patterns[i].Pattern != nil && g.Patterns[i].Pattern.MatchString(s) {
			return &g.Patterns[i]
		}
	}
	return nil
}

// BinaryGuard bounds binary content blocks by size and media type.
type BinaryGuard struct {
	// MaxBytes caps one binary block's payload (base64-decoded length is
	// approximated by the encoded length *3/4 — good enough for a bound).
	MaxBytes int64
	// AllowTypes is a media-type allowlist (prefix match, e.g. "image/",
	// "audio/mpeg"); empty = all types allowed.
	AllowTypes []string
}

// Check reports a violation for one block ("" when the block passes).
func (b *BinaryGuard) Check(mimeType string, encodedLen int) string {
	if b == nil {
		return ""
	}
	if b.MaxBytes > 0 {
		approx := int64(float64(encodedLen) * 3 / 4)
		if approx > b.MaxBytes {
			return fmt.Sprintf("binary block %s ~%d bytes exceeds %d", mimeType, approx, b.MaxBytes)
		}
	}
	if len(b.AllowTypes) > 0 {
		ok := false
		for _, want := range b.AllowTypes {
			if mimeType == want || strings.HasPrefix(mimeType, strings.TrimSuffix(want, "*")) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Sprintf("binary block type %q not in allowlist", mimeType)
		}
	}
	return ""
}

// GuardrailConfig configures each surface independently. A nil guard on a
// surface disables that surface's checks (no rules → nothing to hit);
// Enabled=false disables the whole middleware (the acceptance's "关闭"
// case — content flows untouched).
type GuardrailConfig struct {
	Enabled bool
	Input   *ContentGuard // model input (user text entering reasoning)
	Output  *ContentGuard // model output (assistant text leaving reasoning / reply)
	Tool    *ContentGuard // tool text (string inputs + tool result text)
	Binary  *BinaryGuard  // binary blocks (messages and tool results)
}

// GuardrailMiddleware implements Reply / Reasoning / Acting interceptors.
type GuardrailMiddleware struct {
	Base
	cfg  GuardrailConfig
	hits []GuardrailHit
}

// GuardrailHit is one audited hit (also the test/observability surface —
// nothing is blocked silently).
type GuardrailHit struct {
	Surface string // "input" | "output" | "tool" | "binary"
	Pattern string // pattern name (binary: "size"/"type")
	Detail  string
}

// NewGuardrailMiddleware builds the middleware. Conservative default: hit
// = visible block, never an in-place mutation.
func NewGuardrailMiddleware(cfg GuardrailConfig) *GuardrailMiddleware {
	if cfg.Input == nil && cfg.Output == nil && cfg.Tool == nil && cfg.Binary == nil {
		cfg.Enabled = false // no rules configured: pass through, say so
	}
	return &GuardrailMiddleware{cfg: cfg}
}

// Hits returns the audit trail of every hit so far.
func (m *GuardrailMiddleware) Hits() []GuardrailHit { return append([]GuardrailHit(nil), m.hits...) }

func (m *GuardrailMiddleware) audit(surface, pattern, detail string) {
	m.hits = append(m.hits, GuardrailHit{Surface: surface, Pattern: pattern, Detail: detail})
	logging.FromContext(context.Background()).Info("guardrail: hit",
		"surface", surface, "pattern", pattern, "detail", detail)
}

func (m *GuardrailMiddleware) block(surface, pattern, detail string) error {
	m.audit(surface, pattern, detail)
	return fmt.Errorf("%w: %s %s: %s", ErrGuardrailBlocked, surface, pattern, detail)
}

// OnReply guards the final reply output (text + binary blocks).
func (m *GuardrailMiddleware) OnReply(ctx context.Context, agent Agent, input *ReplyInput, next ReplyNext) (*message.Msg, error) {
	if !m.cfg.Enabled {
		return next(ctx)
	}
	resp, err := next(ctx)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		if hit := m.cfg.Output.Match(resp.GetTextContent()); hit != nil {
			return nil, m.block("output", hit.Name, shortText(resp.GetTextContent()))
		}
		if v := checkBlocks(m.cfg.Binary, resp.Content); v != "" {
			return nil, m.block("binary", "block", v)
		}
	}
	return resp, nil
}

// OnReasoning guards the user input entering one reasoning step and the
// assistant output leaving it.
func (m *GuardrailMiddleware) OnReasoning(ctx context.Context, agent Agent, input *ReasoningInput, next ReasoningNext) (*message.Msg, error) {
	if !m.cfg.Enabled {
		return next(ctx)
	}
	// Input surface: the trailing user message of this step.
	if len(input.Messages) > 0 {
		last := input.Messages[len(input.Messages)-1]
		if last != nil && last.Role == message.RoleUser {
			if hit := m.cfg.Input.Match(last.GetTextContent()); hit != nil {
				return nil, m.block("input", hit.Name, shortText(last.GetTextContent()))
			}
			if v := checkBlocks(m.cfg.Binary, last.Content); v != "" {
				return nil, m.block("binary", "block", v)
			}
		}
	}
	resp, err := next(ctx)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		if hit := m.cfg.Output.Match(resp.GetTextContent()); hit != nil {
			return nil, m.block("output", hit.Name, shortText(resp.GetTextContent()))
		}
		if v := checkBlocks(m.cfg.Binary, resp.Content); v != "" {
			return nil, m.block("binary", "block", v)
		}
	}
	return resp, nil
}

// OnActing guards the tool's string inputs and its result text/binary
// blocks.
func (m *GuardrailMiddleware) OnActing(ctx context.Context, agent Agent, input *ActingInput, next ActingNext) (*tool.Response, error) {
	if !m.cfg.Enabled {
		return next(ctx)
	}
	for k, v := range input.ToolInput {
		if s, ok := v.(string); ok {
			if hit := m.cfg.Tool.Match(s); hit != nil {
				return nil, m.block("tool", hit.Name, k+": "+shortText(s))
			}
		}
	}
	resp, err := next(ctx)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		if v := checkBlocks(m.cfg.Binary, resp.Content); v != "" {
			return nil, m.block("binary", "block", v)
		}
		var texts []string
		for _, b := range resp.Content {
			if tb, ok := b.(*message.TextBlock); ok {
				texts = append(texts, tb.Text)
			}
		}
		if hit := m.cfg.Tool.Match(strings.Join(texts, "\n")); hit != nil {
			return nil, m.block("tool", hit.Name, shortText(strings.Join(texts, "\n")))
		}
	}
	return resp, nil
}

// checkBlocks applies the binary guard to a message's binary blocks.
func checkBlocks(g *BinaryGuard, blocks []message.ContentBlock) string {
	if g == nil {
		return ""
	}
	for _, b := range blocks {
		switch d := b.(type) {
		case *message.ImageBlock:
			data := d.Base64
			if data == "" {
				data = d.URL
			}
			if v := g.Check(d.MimeType, len(data)); v != "" {
				return v
			}
		case *message.AudioBlock:
			data := d.Base64
			if data == "" {
				data = d.URL
			}
			if v := g.Check(d.MimeType, len(data)); v != "" {
				return v
			}
		case *message.DataBlock:
			if d.Source != nil {
				if v := g.Check(d.Source.MediaType, len(d.Source.Data)); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// shortText truncates content for audit lines (the audit shows what hit,
// bounded — not the whole payload).
func shortText(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
