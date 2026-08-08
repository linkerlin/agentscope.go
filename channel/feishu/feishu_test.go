package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/channel"
)

// mockFeishuServer fakes the Feishu Open Platform API.
type mockFeishuServer struct {
	mu         sync.Mutex
	sent       []map[string]any
	tokenCalls int
}

func (m *mockFeishuServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.tokenCalls++
		m.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"tenant_access_token": "test-token", "expire": 7200,
		})
	})
	mux.HandleFunc("/im/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("receive_id_type") != "chat_id" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		m.sent = append(m.sent, body)
		m.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"})
	})
	return mux
}

func newTestChannel(t *testing.T) (*Channel, *mockFeishuServer) {
	t.Helper()
	mock := &mockFeishuServer{}
	srv := httptest.NewServer(mock.handler())
	t.Cleanup(srv.Close)
	c := New("feishu-1", "app-id", "app-secret").WithBaseURL(srv.URL)
	return c, mock
}

func TestTokenRefreshAndCache(t *testing.T) {
	c, mock := newTestChannel(t)
	ctx := context.Background()

	tok1, err := c.token(ctx)
	if err != nil || tok1 != "test-token" {
		t.Fatalf("token1: %v %q", err, tok1)
	}
	// cached — second call should not hit the API again
	tok2, _ := c.token(ctx)
	if tok2 != "test-token" {
		t.Fatalf("token2: %q", tok2)
	}
	mock.mu.Lock()
	calls := mock.tokenCalls
	mock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected 1 token call, got %d (cache broken)", calls)
	}
}

func TestSendText(t *testing.T) {
	c, mock := newTestChannel(t)
	if err := c.SendText(context.Background(), "chat-1", "hello"); err != nil {
		t.Fatal(err)
	}
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(mock.sent))
	}
	msg := mock.sent[0]
	if msg["receive_id"] != "chat-1" || msg["msg_type"] != "text" {
		t.Fatalf("message wrong: %+v", msg)
	}
}

func TestWebhook_URLVerification(t *testing.T) {
	c, _ := newTestChannel(t)
	req := httptest.NewRequest("POST", "/webhook",
		bytes.NewReader([]byte(`{"challenge":"abc123","type":"url_verification"}`)))
	w := httptest.NewRecorder()
	c.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["challenge"] != "abc123" {
		t.Fatalf("challenge not echoed: %v", resp)
	}
}

func TestWebhook_MessageEvent(t *testing.T) {
	c, _ := newTestChannel(t)
	var mu sync.Mutex
	var got []channel.ChannelEvent
	c.emit = func(ev channel.ChannelEvent) error {
		mu.Lock()
		got = append(got, ev)
		mu.Unlock()
		return nil
	}

	payload := `{
	  "type":"event_callback",
	  "event":{
	    "type":"im.message.receive_v1",
	    "message":{"message_id":"m1","message_type":"text","chat_id":"chat-9","content":"{\"text\":\"hello feishu\"}"},
	    "sender":{"sender_id":{"open_id":"user-1"},"sender_type":"user"}
	  }
	}`
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(payload)))
	w := httptest.NewRecorder()
	c.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	ev := got[0]
	if ev.ChatID != "chat-9" || ev.Text != "hello feishu" || ev.ChannelUserID != "user-1" {
		t.Fatalf("event wrong: %+v", ev)
	}
}

func TestWebhook_ImageEventMetadata(t *testing.T) {
	c, _ := newTestChannel(t)
	var got channel.ChannelEvent
	c.emit = func(ev channel.ChannelEvent) error {
		got = ev
		return nil
	}
	payload := `{
	  "type":"event_callback",
	  "event":{
	    "type":"im.message.receive_v1",
	    "message":{"message_id":"m2","message_type":"image","chat_id":"chat-1","content":"{\"image_key\":\"img-123\"}"}
	  }
	}`
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(payload)))
	c.ServeHTTP(httptest.NewRecorder(), req)
	if got.Metadata["image_key"] != "img-123" {
		t.Fatalf("image metadata missing: %+v", got.Metadata)
	}
}

func TestChannelImplementsInterface(t *testing.T) {
	var _ channel.Channel = (*Channel)(nil)
}

func TestSendTextNotStartedToken(t *testing.T) {
	// token failure path: server returns non-zero code
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"code": 10003, "msg": "bad app"})
	}))
	defer srv.Close()
	c := New("x", "app", "secret").WithBaseURL(srv.URL)
	if err := c.SendText(context.Background(), "c", "hi"); err == nil {
		t.Fatal("expected token error")
	}
}
