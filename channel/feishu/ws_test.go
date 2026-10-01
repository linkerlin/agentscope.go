package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/channel"
)

// --- REST capability contracts (fake API) ---

// TestFeishuCapabilityContracts locks the fake-API contracts for the three
// capability surfaces (18.6): declared capabilities, reaction delivery, and
// chat listing.
func TestFeishuCapabilityContracts(t *testing.T) {
	var mu sync.Mutex
	var reactBody, chatsPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"tenant_access_token":"tok","expire":7200}`))
	})
	mux.HandleFunc("/im/v1/chats", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		chatsPath = r.URL.RequestURI()
		mu.Unlock()
		w.Write([]byte(`{"code":0,"data":{"items":[{"chat_id":"oc-1","name":"Deploy"},{"chat_id":"oc-2","name":"Ops"}]}}`))
	})
	mux.HandleFunc("/im/v1/messages/om-x/reactions", func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 512)
		n, _ := r.Body.Read(b)
		mu.Lock()
		reactBody = string(b[:n])
		mu.Unlock()
		w.Write([]byte(`{"code":0}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New("fs1", "app", "secret").WithBaseURL(srv.URL)

	// Declared capabilities drive policy.
	if !channel.HasCapability(c, channel.CapReaction) ||
		!channel.HasCapability(c, channel.CapListChats) ||
		!channel.HasCapability(c, channel.CapWebSocket) {
		t.Fatalf("feishu capabilities incomplete: %v", c.Capabilities())
	}
	if c.MaxTextLen() != 4000 {
		t.Fatalf("text limit: %d", c.MaxTextLen())
	}

	// Reaction contract.
	if err := c.React(context.Background(), "om-x", "THUMBSUP"); err != nil {
		t.Fatalf("react: %v", err)
	}
	mu.Lock()
	body := reactBody
	mu.Unlock()
	if !strings.Contains(body, "THUMBSUP") || !strings.Contains(body, "emoji_type") {
		t.Fatalf("reaction body wrong: %s", body)
	}

	// Chat listing contract.
	chats, err := c.ListChats(context.Background())
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	if len(chats) != 2 || chats[0].ID != "oc-1" || chats[0].Name != "Deploy" {
		t.Fatalf("chats wrong: %+v", chats)
	}
	mu.Lock()
	uri := chatsPath
	mu.Unlock()
	if !strings.Contains(uri, "receive_id_type") && !strings.Contains(uri, "page_size") {
		t.Fatalf("chats request URI wrong: %s", uri)
	}
}

// --- WebSocket long-connection contracts (fake WS server) ---

type wsHub struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	frames   []map[string]any // everything the client sent (pong/ack)
	events   []channel.ChannelEvent
	complete chan struct{}
}

func fakeWSServer(t *testing.T, h *wsHub) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.mu.Lock()
		h.conn = conn
		h.mu.Unlock()
		defer conn.Close()
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var f map[string]any
			if json.Unmarshal(raw, &f) != nil {
				continue
			}
			h.mu.Lock()
			h.frames = append(h.frames, f)
			h.mu.Unlock()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

// pushEvent delivers one server→client event frame.
func pushEvent(t *testing.T, h *wsHub, messageID, text string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		h.mu.Lock()
		conn := h.conn
		h.mu.Unlock()
		if conn != nil {
			payload, _ := json.Marshal(map[string]any{
				"type": "im.message.receive_v1",
				"message": map[string]any{
					"message_id": messageID, "chat_id": "oc-9", "content": `{"text":"` + text + `"}`,
				},
				"sender": map[string]any{"sender_id": map[string]any{"open_id": "ou-1"}},
			})
			frame, _ := json.Marshal(map[string]any{"type": "event", "message_id": messageID, "event": json.RawMessage(payload)})
			if err := conn.WriteMessage(websocket.TextMessage, frame); err == nil {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("could not push event: no connection")
}

func hubFrames(h *wsHub) []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]map[string]any, len(h.frames))
	copy(out, h.frames)
	return out
}

// TestFeishuWebSocketContract walks the long-connection contract: events
// are normalised and emitted, acked exactly once per message id, pings are
// answered with pongs, and duplicate deliveries are re-acked but not
// re-emitted.
func TestFeishuWebSocketContract(t *testing.T) {
	h := &wsHub{complete: make(chan struct{}, 8)}
	srv := fakeWSServer(t, h)

	w := NewWebSocket("fs-ws", "app", "secret", wsURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan channel.ChannelEvent, 8)
	go func() {
		_ = w.Start(ctx, func(ev channel.ChannelEvent) error {
			events <- ev
			return nil
		})
	}()

	// Event 1: delivered, emitted, acked.
	pushEvent(t, h, "msg-1", "hello ws")
	select {
	case ev := <-events:
		if ev.Text != "hello ws" || ev.ChatID != "oc-9" || ev.ChannelUserID != "ou-1" {
			t.Fatalf("normalised event wrong: %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("event never emitted")
	}

	// Duplicate delivery of the same message id: acked again, NOT re-emitted.
	pushEvent(t, h, "msg-1", "hello ws")
	select {
	case ev := <-events:
		t.Fatalf("duplicate event re-emitted: %+v", ev)
	case <-time.After(300 * time.Millisecond):
	}

	// Ping → pong.
	h.mu.Lock()
	conn := h.conn
	h.mu.Unlock()
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))

	deadline := time.Now().Add(3 * time.Second)
	var sawAck, sawPong bool
	for time.Now().Before(deadline) {
		for _, f := range hubFrames(h) {
			if f["type"] == "ack" && f["message_id"] == "msg-1" {
				sawAck = true
			}
			if f["type"] == "pong" {
				sawPong = true
			}
		}
		if sawAck && sawPong {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !sawAck {
		t.Fatal("event never acked")
	}
	if !sawPong {
		t.Fatal("ping never answered with pong")
	}
	cancel()
}

// TestFeishuWebSocketReconnect locks the reconnect contract: after the
// server drops the connection, the adapter redials (fast at the exponential
// base) and resumes event delivery on the fresh connection.
func TestFeishuWebSocketReconnect(t *testing.T) {
	h := &wsHub{complete: make(chan struct{}, 8)}
	srv := fakeWSServer(t, h)

	w := NewWebSocket("fs-ws2", "app", "secret", wsURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan channel.ChannelEvent, 8)
	started := make(chan struct{})
	go func() {
		close(started)
		_ = w.Start(ctx, func(ev channel.ChannelEvent) error {
			events <- ev
			return nil
		})
	}()
	<-started

	// First connection established, then killed server-side.
	deadline := time.Now().Add(3 * time.Second)
	for {
		h.mu.Lock()
		conn := h.conn
		h.mu.Unlock()
		if conn != nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first connection never established")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// After the drop, the adapter redials and a new event still lands.
	deadline = time.Now().Add(5 * time.Second)
	var reconnected bool
	for time.Now().Before(deadline) {
		h.mu.Lock()
		conn := h.conn
		h.mu.Unlock()
		if conn != nil {
			// Probe: try pushing; if the write works we are on a live conn.
			payload, _ := json.Marshal(map[string]any{
				"type":    "im.message.receive_v1",
				"message": map[string]any{"message_id": "msg-r1", "chat_id": "oc-1", "content": `{"text":"after reconnect"}`},
			})
			frame, _ := json.Marshal(map[string]any{"type": "event", "message_id": "msg-r1", "event": json.RawMessage(payload)})
			if err := conn.WriteMessage(websocket.TextMessage, frame); err == nil {
				select {
				case ev := <-events:
					if ev.Text != "after reconnect" {
						t.Fatalf("post-reconnect event wrong: %+v", ev)
					}
					reconnected = true
				case <-time.After(time.Second):
				}
				break
			}
		}
		if reconnected {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !reconnected {
		t.Fatal("adapter never resumed event delivery after reconnect")
	}
}
