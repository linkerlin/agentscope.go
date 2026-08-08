package channel

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// WebhookChannel is a zero-dependency HTTP channel: a POST to its handler is
// normalised into a ChannelEvent and emitted into the gateway. It is the
// fastest way to exercise the full inbound pipeline (and the base pattern for
// platform webhooks like Feishu). SendText is a no-op — webhook replies flow
// back through whatever platform owns the chat in production.
type WebhookChannel struct {
	id   string
	emit func(ChannelEvent) error
	mu   sync.Mutex
	Log  *slog.Logger
}

// NewWebhookChannel creates a webhook channel with the given id.
func NewWebhookChannel(id string) *WebhookChannel {
	return &WebhookChannel{id: id, Log: slog.Default()}
}

// ID returns the channel id.
func (w *WebhookChannel) ID() string { return w.id }

// Start stores the emit callback (no long-lived connection for webhooks).
func (w *WebhookChannel) Start(ctx context.Context, emit func(ChannelEvent) error) error {
	w.mu.Lock()
	w.emit = emit
	w.mu.Unlock()
	return nil
}

// SendText is a no-op for webhook channels (see package comment).
func (w *WebhookChannel) SendText(ctx context.Context, chatID, text string) error {
	return nil
}

// Close clears the emit callback (idempotent).
func (w *WebhookChannel) Close() error {
	w.mu.Lock()
	w.emit = nil
	w.mu.Unlock()
	return nil
}

// webhookPayload is the accepted JSON body for inbound webhook messages.
type webhookPayload struct {
	ChatID    string   `json:"chat_id"`
	ChatName  string   `json:"chat_name,omitempty"`
	UserID    string   `json:"user_id"`
	UserName  string   `json:"user_name,omitempty"`
	MessageID string   `json:"message_id,omitempty"`
	Text      string   `json:"text,omitempty"`
	MediaURLs []string `json:"media_urls,omitempty"`
}

// ServeHTTP normalises the POST body into a ChannelEvent and emits it.
// Returns 200 on accepted, 400 on malformed body.
func (w *WebhookChannel) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var p webhookPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	if p.ChatID == "" {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	w.mu.Lock()
	emit := w.emit
	w.mu.Unlock()
	if emit == nil {
		rw.WriteHeader(http.StatusServiceUnavailable) // not started
		return
	}
	ev := ChannelEvent{
		ChannelID:        w.id,
		ChannelUserID:    p.UserID,
		ChannelUserName:  p.UserName,
		ChatID:           p.ChatID,
		ChatName:         p.ChatName,
		ChannelMessageID: p.MessageID,
		Text:             p.Text,
		MediaURLs:        p.MediaURLs,
		ReceivedAt:       time.Now(),
	}
	if err := emit(ev); err != nil {
		w.Log.Error("webhook channel: emit failed", "error", err, "chat_id", p.ChatID)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}
