// channel/feishu/ws.go adds the WebSocket long-connection inbound mode
// (18.6): instead of exposing an event-subscription webhook, the adapter
// holds one persistent connection to the platform's event gateway and
// receives pushed events. Wire contract (JSON frames):
//
//	→ {"type":"pong"}                     // answer to a server "ping"
//	→ {"type":"ack","message_id":...}     // acknowledge one delivered event
//	← {"type":"event","message_id":..., "event":{...im.message.receive_v1 shape...}}
//
// The frame shape is this repo's stated contract — covered end-to-end by
// the fake WebSocket server in ws_test.go; deployments pointing at the real
// Feishu event gateway adapt the envelope in decodeFrame only. Reconnect is
// exponential (500ms → 30s) after any read error; pings reset a watchdog so
// a silently dead socket is recycled.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/channel"
)

// WS dial/keepalive tuning.
const (
	wsReconnectBase = 500 * time.Millisecond
	wsReconnectMax  = 30 * time.Second
)

// wsPingTimeout is how long without server traffic before the connection is
// considered dead and redialled. A var (not const) so tests can shorten it.
var wsPingTimeout = 45 * time.Second

// WSChannel is the WebSocket-mode Feishu adapter. It reuses the REST sender
// of the webhook Channel (token/SendText/React/ListChats carry over) and
// swaps the inbound path for a long-lived connection.
type WSChannel struct {
	*Channel // embed: outbound REST + capability declaration

	url    string // ws(s) endpoint
	dialer *websocket.Dialer

	mu       sync.Mutex
	emitFn   func(channel.ChannelEvent) error
	sequence int64 // last processed message id guard (duplicate ack suppression)
	lastMsg  map[string]bool
	// pingTimeout snapshots wsPingTimeout at construction so tests can
	// shorten the idle window without racing the reader goroutine on the
	// package-level var.
	pingTimeout time.Duration
}

// NewWebSocket creates the long-connection variant. url defaults to the
// platform event gateway; tests point it at a fake WS server.
func NewWebSocket(id, appID, appSecret, url string) *WSChannel {
	if url == "" {
		url = "wss://open.feishu.cn/open-apis/event/ws"
	}
	return &WSChannel{
		Channel:     New(id, appID, appSecret),
		url:         url,
		dialer:      &websocket.Dialer{HandshakeTimeout: 10 * time.Second},
		lastMsg:     map[string]bool{},
		pingTimeout: wsPingTimeout,
	}
}

// ID reports the ws-mode identity (distinct from the webhook instance).
func (w *WSChannel) ID() string { return w.Channel.ID() + "-ws" }

// Capabilities adds the websocket declaration on top of the REST set.
func (w *WSChannel) Capabilities() []channel.Capability {
	return append(w.Channel.Capabilities(), channel.CapWebSocket)
}

// Start holds the connection for the adapter's lifetime: dial, read frames
// (events → emit + ack; pings → pong), redial with exponential backoff on
// any error. Returns when ctx is cancelled.
func (w *WSChannel) Start(ctx context.Context, emit func(channel.ChannelEvent) error) error {
	w.mu.Lock()
	w.emitFn = emit
	w.mu.Unlock()

	bo := wsReconnectBase
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, _, err := w.dialer.DialContext(ctx, w.url, nil)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			sleepCtx(ctx, bo)
			bo = minDur(bo*2, wsReconnectMax)
			continue
		}
		bo = wsReconnectBase
		if err := w.serveConn(ctx, conn); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		conn.Close()
		// Fall through: redial (fresh backoff base after a healthy run is
		// handled by resetting above).
	}
}

// serveConn reads one connection until an error; each event frame is
// normalised, emitted, and acked.
func (w *WSChannel) serveConn(ctx context.Context, conn *websocket.Conn) error {
	// lastTraffic is written by the read loop and read by the watchdog
	// goroutine — atomic, not a plain variable (race-detector clean).
	var lastTraffic atomic.Int64
	lastTraffic.Store(time.Now().UnixNano())
	gone := make(chan struct{}) // closed by the watchdog when the socket is idle-dead
	stop := make(chan struct{}) // closed on return: stops the watchdog
	var once sync.Once
	closeGone := func() { once.Do(func() { close(gone) }) }
	go func() {
		defer closeGone()
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if ctx.Err() != nil || time.Since(time.Unix(0, lastTraffic.Load())) > w.pingTimeout {
					// Close the socket, not just the signal channel: a
					// ReadMessage parked on a silently-dead peer blocks
					// forever unless the transport is torn down (or the
					// read deadline below fires first — this is the second
					// line of defence).
					_ = conn.Close()
					return
				}
			}
		}
	}()
	defer close(stop)

	// Read deadline as the FIRST line of defence: a parked ReadMessage wakes
	// with a timeout error even if the watchdog goroutine itself is delayed,
	// and the deadline doubles as the idle cut-off.
	_ = conn.SetReadDeadline(time.Now().Add(w.pingTimeout))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-gone:
			return fmt.Errorf("feishu ws: connection idle past watchdog")
		default:
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		lastTraffic.Store(time.Now().UnixNano())
		_ = conn.SetReadDeadline(time.Now().Add(w.pingTimeout))

		var frame struct {
			Type      string          `json:"type"`
			MessageID string          `json:"message_id"`
			Event     json.RawMessage `json:"event"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue // undecodable frame: skip, keep the connection
		}
		switch frame.Type {
		case "ping":
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		case "event":
			ev := w.normalizeWS(frame.Event)
			if w.alreadySeen(frame.MessageID) {
				_ = w.ack(conn, frame.MessageID)
				continue // duplicate delivery: ack, no re-emit
			}
			w.mu.Lock()
			emit := w.emitFn
			w.mu.Unlock()
			if emit != nil {
				if err := emit(ev); err == nil {
					_ = w.ack(conn, frame.MessageID)
				}
				// emit failure: no ack — the platform redelivers.
			}
		}
	}
}

// alreadySeen reports whether the message id was processed before
// (in-process dedup; a fresh process re-acks and re-emits, which is
// at-least-once by contract).
func (w *WSChannel) alreadySeen(id string) bool {
	if id == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lastMsg[id] {
		return true
	}
	w.lastMsg[id] = true
	if len(w.lastMsg) > 4096 {
		w.lastMsg = map[string]bool{} // bounded: reset rather than grow unbounded
	}
	return false
}

func (w *WSChannel) ack(conn *websocket.Conn, messageID string) error {
	if messageID == "" {
		return nil
	}
	return conn.WriteJSON(map[string]any{"type": "ack", "message_id": messageID})
}

// normalizeWS decodes the event payload (same im.message.receive_v1 shape
// as the webhook body's event object).
func (w *WSChannel) normalizeWS(raw json.RawMessage) channel.ChannelEvent {
	var inner struct {
		Type    string `json:"type"`
		Message *struct {
			MessageID   string `json:"message_id"`
			MessageType string `json:"message_type"`
			ChatID      string `json:"chat_id"`
			Content     string `json:"content"`
		} `json:"message"`
		Sender *struct {
			SenderID *struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
		} `json:"sender"`
	}
	ev := channel.ChannelEvent{ChannelID: w.Channel.ID(), ReceivedAt: time.Now()}
	if json.Unmarshal(raw, &inner) != nil {
		return ev
	}
	if inner.Message != nil {
		ev.ChatID = inner.Message.ChatID
		ev.ChannelMessageID = inner.Message.MessageID
		var content map[string]string
		if json.Unmarshal([]byte(inner.Message.Content), &content) == nil {
			ev.Text = strings.TrimSpace(content["text"])
		}
	}
	if inner.Sender != nil && inner.Sender.SenderID != nil {
		ev.ChannelUserID = inner.Sender.SenderID.OpenID
	}
	return ev
}

// Compile-time: the WS adapter is a full Channel (outbound via embedding).
var _ channel.Channel = (*WSChannel)(nil)

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
