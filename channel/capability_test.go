package channel

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubChannel is the minimal Channel for policy tests.
type stubChannel struct {
	sent  []string
	limit int
	caps  []Capability
}

func (s *stubChannel) ID() string { return "stub" }
func (s *stubChannel) Start(ctx context.Context, emit func(ChannelEvent) error) error {
	return nil
}
func (s *stubChannel) SendText(ctx context.Context, chatID, text string) error {
	s.sent = append(s.sent, text)
	return nil
}
func (s *stubChannel) Close() error { return nil }
func (s *stubChannel) MaxTextLen() int {
	if s.limit <= 0 {
		return 10 // default small limit for tests
	}
	return s.limit
}
func (s *stubChannel) Capabilities() []Capability { return s.caps }

func TestSplitLongTextBoundaries(t *testing.T) {
	// No limit: untouched.
	if got := SplitLongText("abc", 0); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("no-limit split wrong: %v", got)
	}
	// Within limit: untouched.
	if got := SplitLongText("abc", 10); len(got) != 1 {
		t.Fatalf("within-limit split wrong: %v", got)
	}
	// Paragraph boundary preferred.
	text := "para one\n\npara two\n\npara three"
	parts := SplitLongText(text, 12)
	if len(parts) < 2 {
		t.Fatalf("paragraph split produced %d parts", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > 12 {
			t.Fatalf("part over limit: %q (%d)", p, len([]rune(p)))
		}
	}
	// Reassembly preserves content.
	joined := strings.Join(parts, "")
	if joined != text {
		t.Fatalf("split lost content: %q", joined)
	}
	// Hard cut when no boundaries: never mid-rune.
	cjk := strings.Repeat("汉", 25)
	hard := SplitLongText(cjk, 10)
	if len(hard) != 3 || hard[2] != strings.Repeat("汉", 5) {
		t.Fatalf("hard cut wrong: %v", hard)
	}
}

func TestDeliverTextPolicy(t *testing.T) {
	ctx := context.Background()
	ch := &stubChannel{}
	long := strings.Repeat("x", 25)

	if err := DeliverText(ctx, ch, "c1", long); err != nil {
		t.Fatal(err)
	}
	if len(ch.sent) != 3 {
		t.Fatalf("25 runes at limit 10 must split into 3, got %d", len(ch.sent))
	}

	// Undeclared limit → no splitting (pre-18.6 behaviour).
	bare := &stubChannel{}
	bare.sent = nil
	_ = bare.MaxTextLen // declared, so use a wrapper without the interface
	noLimit := struct {
		Channel
	}{Channel: &plainChannel{inner: ch}}
	if err := DeliverText(ctx, noLimit, "c1", long); err != nil {
		t.Fatal(err)
	}
	// plainChannel has no TextLimitProvider: single send.
	if len(ch.sent) != 4 { // 3 previous + 1 unsplit
		t.Fatalf("undeclared channel must send unsplit: %d", len(ch.sent)-3)
	}

	// First failure aborts the remainder.
	ch2 := &stubChannel{limit: 10}
	ch2.sent = nil
	failCh := &failAfterOne{inner: ch2}
	if err := DeliverText(ctx, failCh, "c1", long); err == nil {
		t.Fatal("expected abort on send failure")
	}
	if len(ch2.sent) != 1 {
		t.Fatalf("remainder not aborted after failure: %d", len(ch2.sent))
	}
}

type plainChannel struct{ inner Channel }

func (p *plainChannel) ID() string { return p.inner.ID() }
func (p *plainChannel) Start(ctx context.Context, emit func(ChannelEvent) error) error {
	return p.inner.Start(ctx, emit)
}
func (p *plainChannel) SendText(ctx context.Context, chatID, text string) error {
	return p.inner.SendText(ctx, chatID, text)
}
func (p *plainChannel) Close() error { return p.inner.Close() }

type failAfterOne struct{ inner *stubChannel }

func (f *failAfterOne) ID() string { return f.inner.ID() }
func (f *failAfterOne) Start(ctx context.Context, emit func(ChannelEvent) error) error {
	return nil
}
func (f *failAfterOne) MaxTextLen() int { return 10 }
func (f *failAfterOne) SendText(ctx context.Context, chatID, text string) error {
	if len(f.inner.sent) >= 1 {
		return errors.New("boom")
	}
	return f.inner.SendText(ctx, chatID, text)
}
func (f *failAfterOne) Close() error { return nil }

func TestHasCapability(t *testing.T) {
	ch := &stubChannel{caps: []Capability{CapReaction}}
	if !HasCapability(ch, CapReaction) {
		t.Fatal("declared capability not found")
	}
	if HasCapability(ch, CapListChats) {
		t.Fatal("undeclared capability reported present")
	}
	// Undeclared channel: nothing reported.
	if HasCapability(&plainChannel{inner: ch}, CapReaction) {
		t.Fatal("undeclared channel must report nothing")
	}
}
