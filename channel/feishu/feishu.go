// Package feishu implements a Channel adapter for the Feishu (Lark) platform
// using plain HTTP �?the platform has no official Go SDK. It covers the
// tenant-access-token flow, event-subscription webhooks (with URL-verification
// challenge), and REST message sending. Zero new dependencies (stdlib net/http).
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/channel"
)

// apiBase is the Feishu Open Platform API base URL.
const apiBase = "https://open.feishu.cn/open-apis"

// Channel is the Feishu platform adapter. It receives messages via an event
// subscription webhook (ServeHTTP) and sends replies via the REST API.
type Channel struct {
	id        string
	appID     string
	appSecret string
	baseURL   string
	httpc     *http.Client
	Log       *slog.Logger

	mu          sync.Mutex
	emit        func(channel.ChannelEvent) error
	accessToken string
	tokenExpiry time.Time
}

// New creates a Feishu channel. appID/appSecret come from the Feishu Open
// Platform app credentials.
func New(id, appID, appSecret string) *Channel {
	return &Channel{
		id:        id,
		appID:     appID,
		appSecret: appSecret,
		baseURL:   apiBase,
		httpc:     &http.Client{Timeout: 30 * time.Second},
		Log:       slog.Default(),
	}
}

// WithBaseURL overrides the API base (for testing / on-prem).
func (c *Channel) WithBaseURL(url string) *Channel {
	c.baseURL = url
	return c
}

// ID returns the channel instance identifier.
func (c *Channel) ID() string { return c.id }

// Start stores the emit callback. Feishu uses HTTP webhooks rather than a
// long-lived connection, so Start only wires the callback; ServeHTTP is the
// actual entry point.
func (c *Channel) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
	c.mu.Lock()
	c.emit = emit
	c.mu.Unlock()
	return nil
}

// Close clears the emit callback (idempotent).
func (c *Channel) Close() error {
	c.mu.Lock()
	c.emit = nil
	c.mu.Unlock()
	return nil
}

// --- webhook event subscription ---

// webhookPayload is the envelope Feishu sends to the event-subscription URL.
type webhookPayload struct {
	Challenge string `json:"challenge,omitempty"`
	Type      string `json:"type,omitempty"`
	Event     *struct {
		Type    string `json:"type,omitempty"`
		Message *struct {
			MessageID   string `json:"message_id,omitempty"`
			MessageType string `json:"message_type,omitempty"`
			ChatID      string `json:"chat_id,omitempty"`
			Content     string `json:"content,omitempty"`
		} `json:"message,omitempty"`
		Sender *struct {
			SenderID *struct {
				OpenID string `json:"open_id,omitempty"`
			} `json:"sender_id,omitempty"`
			SenderType string `json:"sender_type,omitempty"`
		} `json:"sender,omitempty"`
	} `json:"event,omitempty"`
}

// ServeHTTP handles the event-subscription endpoint:
//   - url_verification �?echoes the challenge (proving URL ownership)
//   - im.message.receive_v1 �?normalises into a ChannelEvent and emits
func (c *Channel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p webhookPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if p.Challenge != "" {
		// URL verification handshake.
		writeJSON(w, map[string]any{"challenge": p.Challenge})
		return
	}
	if p.Type != "event_callback" || p.Event == nil || p.Event.Message == nil {
		w.WriteHeader(http.StatusOK) // ack unhandled events
		return
	}
	ev := c.normalize(p)
	c.mu.Lock()
	emit := c.emit
	c.mu.Unlock()
	if emit != nil {
		_ = emit(ev)
	}
	w.WriteHeader(http.StatusOK)
}

// normalize converts a Feishu event payload into a ChannelEvent. Message
// content is a JSON string such as {"text":"hello"}.
func (c *Channel) normalize(p webhookPayload) channel.ChannelEvent {
	ev := channel.ChannelEvent{
		ChannelID:        c.id,
		ChatID:           p.Event.Message.ChatID,
		ChannelMessageID: p.Event.Message.MessageID,
		ReceivedAt:       time.Now(),
	}
	if s := p.Event.Sender; s != nil && s.SenderID != nil {
		ev.ChannelUserID = s.SenderID.OpenID
	}
	// message content is JSON: {"text":"..."} or {"image_key":"..."}
	var content map[string]string
	if json.Unmarshal([]byte(p.Event.Message.Content), &content) == nil {
		if text, ok := content["text"]; ok {
			ev.Text = text
		}
		if key, ok := content["image_key"]; ok && key != "" {
			ev.Metadata = map[string]any{"image_key": key}
		}
	}
	return ev
}

// --- outbound messaging ---

// token returns a valid tenant_access_token, refreshing lazily (2h TTL).
func (c *Channel) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	body, _ := json.Marshal(map[string]string{
		"app_id": c.appID, "app_secret": c.appSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu: token: %w", err)
	}
	defer resp.Body.Close()
	var data struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Token  string `json:"tenant_access_token"`
		Expire int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.Code != 0 || data.Token == "" {
		return "", fmt.Errorf("feishu: token failed code=%d msg=%s", data.Code, data.Msg)
	}
	c.accessToken = data.Token
	c.tokenExpiry = time.Now().Add(time.Duration(data.Expire) * time.Second)
	if data.Expire == 0 {
		c.tokenExpiry = time.Now().Add(2 * time.Hour) // default TTL
	}
	return c.accessToken, nil
}

// SendText sends a plain-text message to a chat.
func (c *Channel) SendText(ctx context.Context, chatID, text string) error {
	return c.sendMessage(ctx, chatID, "text", map[string]any{"text": text})
}

// sendMessage posts a message of the given type to a chat.
func (c *Channel) sendMessage(ctx context.Context, chatID, msgType string, content map[string]any) error {
	tok, err := c.token(ctx)
	if err != nil {
		return err
	}
	contentJSON, _ := json.Marshal(content)
	body, _ := json.Marshal(map[string]any{
		"receive_id": chatID,
		"msg_type":   msgType,
		"content":    string(contentJSON),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: send: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var data struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &data); err == nil && data.Code != 0 {
		return fmt.Errorf("feishu: send failed code=%d msg=%s", data.Code, data.Msg)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("feishu: send status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

// compile-time assertion that Channel implements channel.Channel.
var _ channel.Channel = (*Channel)(nil)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
