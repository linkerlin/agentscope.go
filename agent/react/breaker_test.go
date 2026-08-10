package react

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/linkerlin/agentscope.go/message"
)

// Direct unit tests for the consecutive-failure breaker helpers. The
// integration tests (TestReActAgent_ConsecutiveToolFailure*) cover the full
// loop; these pin the pure-function edge cases.

func failSig(tool string) failureSignal {
	return failureSignal{ToolName: tool, IsError: true, ErrText: "boom"}
}

func okSig(tool string) failureSignal {
	return failureSignal{ToolName: tool, IsError: false}
}

func TestConsecutiveFailureBreaker_Update(t *testing.T) {
	t.Run("trips after N consecutive failures of the same tool", func(t *testing.T) {
		b := newConsecutiveFailureBreaker(3)
		for i := 0; i < 2; i++ {
			if name, _, _ := b.update([]failureSignal{failSig("web_fetch")}); name != "" {
				t.Fatalf("must not trip before threshold")
			}
		}
		name, count, reason := b.update([]failureSignal{failSig("web_fetch")})
		if name != "web_fetch" || count != 3 || reason != "boom" {
			t.Fatalf("expected trip on web_fetch count=3 reason=boom, got %q %d %q", name, count, reason)
		}
	})

	t.Run("success resets that tool's count", func(t *testing.T) {
		b := newConsecutiveFailureBreaker(3)
		b.update([]failureSignal{failSig("web_fetch"), failSig("web_fetch")})
		b.update([]failureSignal{okSig("web_fetch")})
		if name, _, _ := b.update([]failureSignal{failSig("web_fetch")}); name != "" {
			t.Fatalf("success must reset the count: unexpected trip on %q", name)
		}
	})

	t.Run("tools are counted independently", func(t *testing.T) {
		b := newConsecutiveFailureBreaker(3)
		// a keeps failing; b's successes reset b's count. a must trip at 3
		// while b never does.
		b.update([]failureSignal{failSig("a"), failSig("b")})
		b.update([]failureSignal{failSig("a"), okSig("b")})
		name, count, _ := b.update([]failureSignal{failSig("a"), okSig("b")})
		if name != "a" || count != 3 {
			t.Fatalf("expected a to trip at 3 (b reset by successes), got %q %d", name, count)
		}
	})

	t.Run("text-smuggled error counts as failure", func(t *testing.T) {
		b := newConsecutiveFailureBreaker(1)
		smuggled := failureSignal{
			ToolName: "mcp_tool",
			IsError:  false, // success channel, but text says error:
			Blocks:   []message.ContentBlock{message.NewTextBlock("error: connection refused")},
		}
		name, count, reason := b.update([]failureSignal{smuggled})
		if name != "mcp_tool" || count != 1 || reason != "error: connection refused" {
			t.Fatalf("expected trip via smuggled text, got %q %d %q", name, count, reason)
		}
	})

	t.Run("threshold <= 0 never trips", func(t *testing.T) {
		for _, thr := range []int{0, -1} {
			b := newConsecutiveFailureBreaker(thr)
			if name, _, _ := b.update([]failureSignal{failSig("x"), failSig("x")}); name != "" {
				t.Fatalf("threshold %d must never trip", thr)
			}
		}
	})

	t.Run("nil breaker never trips", func(t *testing.T) {
		var b *consecutiveFailureBreaker
		if name, _, _ := b.update([]failureSignal{failSig("x")}); name != "" {
			t.Fatalf("nil breaker must never trip")
		}
	})

	t.Run("empty failure list is a no-op", func(t *testing.T) {
		b := newConsecutiveFailureBreaker(1)
		if name, _, _ := b.update(nil); name != "" {
			t.Fatalf("empty signals must not trip")
		}
	})
}

func TestBreakerFinalMessage(t *testing.T) {
	msg := breakerFinalMessage("agent-1", "web_fetch", 3, "HTTP 429 Too Many Requests")
	text := msg.GetTextContent()
	if !strings.Contains(text, "web_fetch") || !strings.Contains(text, "3") {
		t.Fatalf("message must name the tool and count: %q", text)
	}
	if !strings.Contains(text, "HTTP 429") {
		t.Fatalf("message must carry the reason: %q", text)
	}

	// Empty reason falls back to a default.
	msg2 := breakerFinalMessage("agent-1", "tool-x", 2, "   ")
	if !strings.Contains(msg2.GetTextContent(), "未知错误") {
		t.Fatalf("empty reason must fall back: %q", msg2.GetTextContent())
	}
}

func TestBreakerFinalMessage_TruncatesRunes(t *testing.T) {
	// 250 multi-byte runes (each CJK char is 3 UTF-8 bytes) — must truncate to
	// 200 runes without splitting a rune.
	long := strings.Repeat("错", 250)
	msg := breakerFinalMessage("a", "t", 3, long)
	text := msg.GetTextContent()
	// Extract the reason portion (between "最近错误: " and "），已停止重试").
	start := strings.Index(text, "最近错误: ")
	end := strings.Index(text, "），已停止重试")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("message missing reason markers: %q", text)
	}
	reason := text[start+len("最近错误: ") : end]
	rs := []rune(reason)
	if len(rs) != 201 { // 200 truncated + ellipsis
		t.Fatalf("reason not truncated to 200+ellipsis: %d runes", len(rs))
	}
	if !utf8.ValidString(reason) {
		t.Fatalf("truncation split a multi-byte rune: %q", reason)
	}
	if !strings.HasSuffix(reason, "…") {
		t.Fatalf("truncated reason must end with ellipsis: %q", reason)
	}
}

func TestToolResultLooksLikeError(t *testing.T) {
	cases := []struct {
		name   string
		blocks []message.ContentBlock
		want   bool
	}{
		{"explicit error marker", []message.ContentBlock{message.NewTextBlock("error: connection refused")}, true},
		{"case-insensitive", []message.ContentBlock{message.NewTextBlock("Error: boom")}, true},
		{"leading whitespace", []message.ContentBlock{message.NewTextBlock("   error: boom")}, true},
		{"multiple blocks never match", []message.ContentBlock{message.NewTextBlock("error: a"), message.NewTextBlock("b")}, false},
		{"non-text block never matches", []message.ContentBlock{message.NewImageBlock("", "", "")}, false},
		{"not an error prefix", []message.ContentBlock{message.NewTextBlock("errorX: not a marker")}, false},
		{"normal result", []message.ContentBlock{message.NewTextBlock("everything fine")}, false},
		{"empty blocks", nil, false},
	}
	for _, tc := range cases {
		got, _ := toolResultLooksLikeError(tc.blocks)
		if got != tc.want {
			t.Errorf("%s: toolResultLooksLikeError = %v, want %v", tc.name, got, tc.want)
		}
	}
}
