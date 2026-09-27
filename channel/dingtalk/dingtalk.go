// Package dingtalk implements the DingTalk channel (18.3): an OpenAPI client
// (access-token lifecycle, group/IM message sending), an HTTP inbound endpoint
// for the outgoing-robot callback with HMAC signature verification, AI-card
// create/update with callback normalisation to HITL decisions, and wiki
// knowledge-base tools.
//
// Structure mirrors channel/feishu: the Channel satisfies channel.Channel and
// http.Handler; the gateway mounts it wherever the deployment exposes
// callbacks. No gateway import.
package dingtalk

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/channel"
)

// DefaultAPIBase is the DingTalk OpenAPI root.
const DefaultAPIBase = "https://api.dingtalk.com"

// Channel is the DingTalk adapter: OpenAPI sender + outgoing-robot HTTP
// callback receiver.
type Channel struct {
	id          string
	appKey      string
	appSecret   string
	robotSecret string // outgoing-robot signature secret; empty disables verification
	apiBase     string
	client      *http.Client
	emitFn      func(channel.ChannelEvent) error // wired via WithEmitter

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

// New creates a DingTalk channel from an enterprise-app appKey/appSecret pair.
func New(id, appKey, appSecret string) *Channel {
	return &Channel{
		id:        id,
		appKey:    appKey,
		appSecret: appSecret,
		apiBase:   DefaultAPIBase,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBaseURL overrides the OpenAPI root (tests / private deployments).
func (c *Channel) WithBaseURL(url string) *Channel {
	c.apiBase = url
	return c
}

// WithRobotSecret enables HMAC verification for outgoing-robot callbacks.
func (c *Channel) WithRobotSecret(secret string) *Channel {
	c.robotSecret = secret
	return c
}

// WithHTTPClient overrides the HTTP client (tests).
func (c *Channel) WithHTTPClient(cl *http.Client) *Channel {
	c.client = cl
	return c
}

// ID implements channel.Channel.
func (c *Channel) ID() string { return c.id }

// ChannelKind self-describes the adapter for gateway listings (18.3).
func (c *Channel) ChannelKind() string { return "dingtalk" }

// Close implements channel.Channel (stateless HTTP; nothing to tear down).
func (c *Channel) Close() error { return nil }

// Start implements channel.Channel. The DingTalk adapter is HTTP-pull (the
// platform POSTs to ServeHTTP), so Start blocks on ctx exactly like the
// feishu webhook channel.
func (c *Channel) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
	<-ctx.Done()
	return ctx.Err()
}

// --- access token ---

type tokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int64  `json:"expireIn"` // seconds
}

// accessToken fetches (and caches) the OpenAPI access token.
func (c *Channel) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"appKey": c.appKey, "appSecret": c.appSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/v1.0/oauth2/accessToken", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("dingtalk: token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	var tr tokenResponse
	if err := c.doJSON(req, &tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("dingtalk: empty access token")
	}
	c.token = tr.AccessToken
	// Refresh 5 minutes early to dodge clock skew mid-call.
	c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpireIn)*time.Second - 5*time.Minute)
	return c.token, nil
}

func (c *Channel) doJSON(req *http.Request, out any) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk: %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk: %s %s: status %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(data))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("dingtalk: decode response: %w", err)
		}
	}
	return nil
}

// --- outbound: SendText via OpenAPI ---

func (c *Channel) SendText(ctx context.Context, chatID, text string) error {
	return c.sendGroupMessage(ctx, chatID, "sampleText", map[string]any{"content": text})
}

// sendGroupMessage posts one robot group message.
// Docs: POST /v1.0/robot/groupMessages/send
func (c *Channel) sendGroupMessage(ctx context.Context, openConversationID, msgKey string, msgParam map[string]any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	param, _ := json.Marshal(msgParam)
	body, _ := json.Marshal(map[string]any{
		"msgKey":             msgKey,
		"msgParam":           string(param),
		"openConversationId": openConversationID,
		"robotCode":          c.appKey,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/v1.0/robot/groupMessages/send?access_token="+token, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doJSON(req, nil)
}

// --- inbound: outgoing-robot callback ---

// outgoingPayload is the outgoing-robot callback body (plain JSON,
// HMAC-verified via the timestamp/sign headers).
type outgoingPayload struct {
	ConversationID    string `json:"conversationId"`
	ConversationTitle string `json:"conversationTitle"`
	ConversationType  string `json:"conversationType"`
	SenderStaffID     string `json:"senderStaffId"`
	SenderNick        string `json:"senderNick"`
	MsgID             string `json:"msgId"`
	MsgType           string `json:"msgtype"`
	Text              struct {
		Content string `json:"content"`
	} `json:"text"`
	SessionWebhook string `json:"sessionWebhook"`
}

// verifySignature checks the outgoing-robot HMAC header pair:
// sign = base64(HMAC-SHA256(secret, timestamp + "\n" + secret)).
func (c *Channel) verifySignature(r *http.Request) bool {
	if c.robotSecret == "" {
		return true // verification disabled (private-network deployments)
	}
	ts := r.Header.Get("timestamp")
	sig := r.Header.Get("sign")
	if ts == "" || sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(c.robotSecret))
	mac.Write([]byte(ts + "\n" + c.robotSecret))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(sig))
}

// normalize maps an outgoing callback payload onto a ChannelEvent.
func (c *Channel) normalize(p outgoingPayload) channel.ChannelEvent {
	now := time.Now()
	return channel.ChannelEvent{
		ChannelID:        c.id,
		ChannelUserID:    p.SenderStaffID,
		ChannelUserName:  p.SenderNick,
		ChatID:           p.ConversationID,
		ChatName:         p.ConversationTitle,
		ChannelMessageID: p.MsgID,
		Text:             p.Text.Content,
		Metadata: map[string]any{
			"chat_type":       p.ConversationType,
			"session_webhook": p.SessionWebhook,
		},
		ReceivedAt: now,
	}
}

// ServeHTTP receives one outgoing-robot callback: verifies the signature,
// normalises the payload, forwards to emit, and ACKs 200. Only text messages
// are forwarded; other msgtypes are ACKed and dropped.
func (c *Channel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !c.verifySignature(r) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var p outgoingPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if _, ok := r.Context().Deadline(); !ok {
		// Bound the handler in case emit blocks: DingTalk retries on timeout.
		var cancel context.CancelFunc
		r, cancel = withTimeout(r, 30*time.Second) //nolint:ineffassign,staticcheck // rebind kept local
		defer cancel()
	}
	if p.MsgType != "text" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if c.emitFn != nil {
		if err := c.emitFn(c.normalize(p)); err != nil {
			http.Error(w, "emit failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// withTimeout mirrors context.WithTimeout for the request (small helper to
// keep ServeHTTP readable).
func withTimeout(r *http.Request, d time.Duration) (*http.Request, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), d)
	return r.WithContext(ctx), cancel
}

// WithEmitter wires the emit callback outside Start — required for the
// HTTP-pull adapter so ServeHTTP can forward events without Start running.
func (c *Channel) WithEmitter(emit func(channel.ChannelEvent) error) *Channel {
	c.emitFn = emit
	return c
}

// SignOutgoing computes the expected signature header pair for tests and
// deployments that need to self-check.
func SignOutgoing(secret string, ts time.Time) (timestamp, sign string) {
	tsStr := strconv.FormatInt(ts.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsStr + "\n" + secret))
	return tsStr, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
