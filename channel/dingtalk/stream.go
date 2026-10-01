// channel/dingtalk/stream.go realises the streaming-card half of 18.3: an
// agent's event stream is written into one DingTalk AI card, throttled —
// text deltas accumulate and the card API is hit at most once per interval —
// and the card always lands in a terminal state (finished / failed), so a
// replayed event sequence produces a deterministic, bounded number of API
// calls.
package dingtalk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/event"
)

// DefaultCardStreamInterval is the update throttle window. DingTalk's card
// API tolerates roughly one update per second per instance; a full token
// stream would be hundreds of calls per turn.
const DefaultCardStreamInterval = time.Second

// CardStreamWriter turns an agent event stream into one throttled AI card.
type CardStreamWriter struct {
	sender    *CardSender
	chatID    string
	trackID   string
	interval  time.Duration
	nowFn     func() time.Time // injectable clock (tests)
	maxCardMs int              // response preview cap inside cardData

	mu        sync.Mutex
	created   bool
	buf       strings.Builder
	dirty     bool
	lastFlush time.Time
	updates   int
	finished  bool
}

// NewCardStreamWriter creates a writer that owns card instance trackID in
// chatID. Call Start once, WriteEvent per stream event, Finish exactly once.
func NewCardStreamWriter(sender *CardSender, chatID, trackID string) *CardStreamWriter {
	return &CardStreamWriter{
		sender:    sender,
		chatID:    chatID,
		trackID:   trackID,
		interval:  DefaultCardStreamInterval,
		nowFn:     time.Now,
		maxCardMs: 4000,
	}
}

// WithInterval overrides the throttle window (tests shrink it).
func (w *CardStreamWriter) WithInterval(d time.Duration) *CardStreamWriter {
	if d > 0 {
		w.interval = d
	}
	return w
}

// WithClock injects the clock (deterministic replay tests).
func (w *CardStreamWriter) WithClock(now func() time.Time) *CardStreamWriter {
	if now != nil {
		w.nowFn = now
	}
	return w
}

// Updates reports how many card-update API calls were made (test observer).
func (w *CardStreamWriter) Updates() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.updates
}

// Start creates and delivers the initial card.
func (w *CardStreamWriter) Start(ctx context.Context, cardTemplateID, title string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.sender.CreateAndDeliver(ctx, w.chatID, cardTemplateID, w.trackID,
		map[string]string{"content": "", "status": "running", "title": title}); err != nil {
		return err
	}
	w.created = true
	w.lastFlush = w.nowFn()
	return nil
}

// WriteEvent folds one agent event into the card buffer. Text deltas
// accumulate; only when the throttle window has elapsed is the buffer
// flushed to the card API. Non-text events are ignored.
func (w *CardStreamWriter) WriteEvent(ctx context.Context, ev event.AgentEvent) error {
	delta, ok := ev.(*event.TextBlockDeltaEvent)
	if !ok {
		return nil
	}
	w.mu.Lock()
	if w.finished || !w.created {
		w.mu.Unlock()
		return nil
	}
	w.buf.WriteString(delta.Delta)
	w.dirty = true
	now := w.nowFn()
	if now.Sub(w.lastFlush) < w.interval {
		w.mu.Unlock()
		return nil // throttled: stay in the buffer
	}
	w.lastFlush = now
	content := w.buf.String()
	w.dirty = false
	w.updates++
	w.mu.Unlock()
	return w.sender.UpdateCard(ctx, w.trackID, w.cardData(content, "running"))
}

// Finish flushes whatever remains and writes the terminal state: "finished"
// on success, "failed" with the error text otherwise. Idempotent: a second
// Finish is a no-op (the card must end in exactly one terminal state).
func (w *CardStreamWriter) Finish(ctx context.Context, runErr error) error {
	w.mu.Lock()
	if w.finished || !w.created {
		w.mu.Unlock()
		return nil
	}
	w.finished = true
	status := "finished"
	content := w.buf.String()
	if runErr != nil {
		status = "failed"
		if content == "" {
			content = "turn failed"
		}
	}
	w.updates++
	w.mu.Unlock()
	data := w.cardData(content, status)
	if runErr != nil {
		data["error"] = truncateCardText(runErr.Error(), 300)
	}
	return w.sender.UpdateCard(ctx, w.trackID, data)
}

func (w *CardStreamWriter) cardData(content, status string) map[string]string {
	return map[string]string{
		"content": truncateCardText(content, w.maxCardMs),
		"status":  status,
	}
}

func truncateCardText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Drain consumes an entire event channel through the writer and finishes it
// — the gateway bridge's per-turn loop (18.3). The returned error is the
// first terminal error seen on the stream, so Finish marks the card failed.
func (w *CardStreamWriter) Drain(ctx context.Context, ch <-chan event.AgentEvent) error {
	var runErr error
	for ev := range ch {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			runErr = fmt.Errorf("%s", e.Err)
			continue
		}
		if err := w.WriteEvent(ctx, ev); err != nil {
			// Card API hiccups must not kill the turn: keep draining.
			continue
		}
	}
	return w.Finish(ctx, runErr)
}
