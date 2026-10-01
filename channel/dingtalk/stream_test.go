package dingtalk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
)

// TestCardStreamWriterThrottledReplay locks the 18.3 streaming-card
// contract against a recorded event replay: deltas inside one throttle
// window buffer (zero card API calls), the window close flushes exactly
// once, Finish writes the terminal state exactly once, and post-finish
// events are ignored.
func TestCardStreamWriterThrottledReplay(t *testing.T) {
	f := newFakeCardAPI(t)
	sender := NewCardSender(New("dt-1", "app-key", "app-secret").WithBaseURL(f.srv.URL))

	w := NewCardStreamWriter(sender, "chat-1", "track-1").WithInterval(time.Second)
	t0 := time.Unix(0, 0)
	clock := t0
	w.WithClock(func() time.Time { return clock })

	ctx := context.Background()
	if err := w.Start(ctx, "tpl-1", "demo"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// 50 deltas inside ONE window: all buffered, zero API calls.
	for i := 0; i < 50; i++ {
		if err := w.WriteEvent(ctx, event.NewTextBlockDelta("r", 0, "tok ")); err != nil {
			t.Fatal(err)
		}
	}
	if got := w.Updates(); got != 0 {
		t.Fatalf("updates inside one window: %d, want 0 (all buffered)", got)
	}

	// Advance past the window: the next delta flushes the accumulated text.
	clock = t0.Add(2 * time.Second)
	if err := w.WriteEvent(ctx, event.NewTextBlockDelta("r", 0, "end")); err != nil {
		t.Fatal(err)
	}
	if got := w.Updates(); got != 1 {
		t.Fatalf("updates after window close: %d, want 1", got)
	}
	f.mu.Lock()
	flushed := f.bodies["/v1.0/card/instances"]
	f.mu.Unlock()
	// The window-closing delta is folded into the same flush — buffered
	// text and the triggering delta land together.
	if !strings.Contains(flushed, "tok tok") || !strings.Contains(flushed, "end") {
		t.Fatalf("flushed card data wrong: %s", flushed)
	}

	// Finish: terminal state, exactly once (double finish is a no-op).
	if err := w.Finish(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := w.Updates(); got != 2 {
		t.Fatalf("updates after double finish: %d, want 2 (flush + terminal)", got)
	}
	f.mu.Lock()
	terminal := f.bodies["/v1.0/card/instances"]
	f.mu.Unlock()
	var data struct {
		CardData struct {
			CardParamMap map[string]string `json:"cardParamMap"`
		} `json:"cardData"`
	}
	if err := json.Unmarshal([]byte(terminal), &data); err != nil {
		t.Fatalf("terminal body: %v (%s)", err, terminal)
	}
	if data.CardData.CardParamMap["status"] != "finished" {
		t.Fatalf("terminal status: %q", data.CardData.CardParamMap["status"])
	}
	if !strings.Contains(data.CardData.CardParamMap["content"], "end") {
		t.Fatal("finish did not flush the buffered tail")
	}

	// Events after Finish are ignored.
	_ = w.WriteEvent(ctx, event.NewTextBlockDelta("r", 0, "late"))
	if got := w.Updates(); got != 2 {
		t.Fatalf("post-finish events leaked: updates=%d", got)
	}
}

// TestCardStreamWriterFailureTerminal locks the failed terminal via Drain:
// an error stream ends with status "failed" and the error text on the card.
func TestCardStreamWriterFailureTerminal(t *testing.T) {
	f := newFakeCardAPI(t)
	sender := NewCardSender(New("dt-1", "app-key", "app-secret").WithBaseURL(f.srv.URL))

	w := NewCardStreamWriter(sender, "chat-1", "track-1")
	clock := time.Unix(0, 0)
	w.WithClock(func() time.Time { return clock })
	ctx := context.Background()
	if err := w.Start(ctx, "tpl-1", "demo"); err != nil {
		t.Fatal(err)
	}

	chEv := make(chan event.AgentEvent, 2)
	chEv <- event.NewTextBlockDelta("r", 0, "partial answer")
	chEv <- event.NewError("r", context.DeadlineExceeded)
	close(chEv)

	if err := w.Drain(ctx, chEv); err != nil {
		t.Fatalf("drain: %v", err)
	}
	f.mu.Lock()
	terminal := f.bodies["/v1.0/card/instances"]
	f.mu.Unlock()
	if !strings.Contains(terminal, `"failed"`) {
		t.Fatalf("terminal status not failed: %s", terminal)
	}
	if !strings.Contains(terminal, "context deadline exceeded") {
		t.Fatalf("error text missing from card: %s", terminal)
	}
}

// TestCardStreamWriterBoundedUpdates is the replay guarantee: a
// thousand-delta stream over a handful of windows costs O(windows) card API
// calls, never O(deltas).
func TestCardStreamWriterBoundedUpdates(t *testing.T) {
	f := newFakeCardAPI(t)
	sender := NewCardSender(New("dt-1", "app-key", "app-secret").WithBaseURL(f.srv.URL))

	w := NewCardStreamWriter(sender, "chat-1", "track-1").WithInterval(time.Second)
	t0 := time.Unix(0, 0)
	clock := t0
	w.WithClock(func() time.Time { return clock })
	ctx := context.Background()
	if err := w.Start(ctx, "tpl", "x"); err != nil {
		t.Fatal(err)
	}

	const windows = 5
	for i := 0; i < 1000; i++ {
		if i > 0 && i%200 == 0 {
			clock = clock.Add(time.Second) // close one window per 200 deltas
		}
		_ = w.WriteEvent(ctx, event.NewTextBlockDelta("r", 0, "x"))
	}
	if err := w.Finish(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if w.Updates() > windows+2 {
		t.Fatalf("card API called %d times for %d windows — throttle leaked", w.Updates(), windows)
	}
}
