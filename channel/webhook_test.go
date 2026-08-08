package channel

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// recordingChannel is a Channel that records emitted events + sent texts.
type recordingChannel struct {
	id   string
	emit func(ChannelEvent) error
	mu   sync.Mutex
	got  []ChannelEvent
	sent []string
}

func (r *recordingChannel) ID() string { return r.id }
func (r *recordingChannel) Start(ctx context.Context, emit func(ChannelEvent) error) error {
	r.mu.Lock()
	r.emit = emit
	r.mu.Unlock()
	<-ctx.Done()
	return nil
}
func (r *recordingChannel) SendText(ctx context.Context, chatID, text string) error {
	r.mu.Lock()
	r.sent = append(r.sent, text)
	r.mu.Unlock()
	return nil
}
func (r *recordingChannel) Close() error { return nil }

func (r *recordingChannel) record(ev ChannelEvent) {
	r.mu.Lock()
	r.got = append(r.got, ev)
	r.mu.Unlock()
}

func TestWebhookChannel_FullLoop(t *testing.T) {
	rec := &recordingChannel{id: "wh-1"}
	router := NewChatRouter(RouteTable{
		ChannelID: "wh-1",
		Bindings:  []Binding{{ChatIDPrefix: "dev-", AgentID: "dev-agent", SessionPrefix: "dev-"}},
	})
	runner := &stubRunner{}
	g := NewGateway(router, runner)
	// wire rec as the "owning channel" the runner would use; for this test we
	// assert the event reached the runner (reply path covered in gateway tests).
	_ = rec

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rec.Start(ctx, func(ev ChannelEvent) error { return g.HandleEvent(context.Background(), ev) })
	time.Sleep(50 * time.Millisecond)

	// POST a webhook message
	body := bytes.NewReader([]byte(`{"chat_id":"dev-42","user_id":"u1","user_name":"Alice","text":"hello"}`))
	req := httptest.NewRequest("POST", "/webhook", body)
	w := httptest.NewRecorder()
	wh := NewWebhookChannel("wh-1")
	wh.mu.Lock()
	wh.emit = func(ev ChannelEvent) error { return g.HandleEvent(context.Background(), ev) }
	wh.mu.Unlock()
	wh.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("webhook status = %d", w.Code)
	}
	if len(runner.turns) != 1 {
		t.Fatalf("runner turns = %d, want 1", len(runner.turns))
	}
	ev := runner.turns[0]
	if ev.Text != "hello" || ev.ChannelUserID != "u1" || ev.ChatID != "dev-42" {
		t.Fatalf("event normalized wrong: %+v", ev)
	}
	if runner.lastAgent != "dev-agent" {
		t.Fatalf("agent = %q, want dev-agent", runner.lastAgent)
	}
}

func TestWebhookChannel_BadRequest(t *testing.T) {
	wh := NewWebhookChannel("wh-1")
	// not started: emit is nil → 503
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(`{"chat_id":"x","text":"hi"}`)))
	w := httptest.NewRecorder()
	wh.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("not-started status = %d, want 503", w.Code)
	}
	// malformed json → 400
	req2 := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(`not-json`)))
	w2 := httptest.NewRecorder()
	wh.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d, want 400", w2.Code)
	}
	// missing chat_id → 400
	req3 := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(`{"text":"hi"}`)))
	w3 := httptest.NewRecorder()
	wh.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("no chat_id status = %d, want 400", w3.Code)
	}
	// wrong method → 405
	req4 := httptest.NewRequest("GET", "/webhook", nil)
	w4 := httptest.NewRecorder()
	wh.ServeHTTP(w4, req4)
	if w4.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", w4.Code)
	}
}
