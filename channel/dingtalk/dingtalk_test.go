package dingtalk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linkerlin/agentscope.go/channel"
)

// fakeDingtalkAPI stands in for the OpenAPI: it records requests and serves
// canned token/message responses.
type fakeAPI struct {
	mu       sync.Mutex
	bodies   []string
	paths    []string
	msgs     int
	tokens   int
	srv      *httptest.Server
	msgReply string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{msgReply: "{}"}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/oauth2/accessToken", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.tokens++
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		w.Write([]byte(`{"accessToken":"tok-1","expireIn":7200}`))
	})
	mux.HandleFunc("/v1.0/robot/groupMessages/send", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		f.mu.Lock()
		f.msgs++
		f.bodies = append(f.bodies, string(buf[:n]))
		f.mu.Unlock()
		w.Write([]byte(f.msgReply))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// TestChannel_SendText_TokenAndPayload verifies the OpenAPI path end to end:
// token fetched once, group message carries msgKey/content/robotCode.
func TestChannel_SendText_TokenAndPayload(t *testing.T) {
	f := newFakeAPI(t)
	ch := New("dt-1", "app-key", "app-secret").WithBaseURL(f.srv.URL)

	require.NoError(t, ch.SendText(context.Background(), "cid-001", "hello dingtalk"))
	require.NoError(t, ch.SendText(context.Background(), "cid-001", "again"))

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, 1, f.tokens, "token must be cached across sends")
	require.Len(t, f.bodies, 2)
	assert.Contains(t, f.bodies[0], `"msgKey":"sampleText"`)
	assert.Contains(t, f.bodies[0], "hello dingtalk")
	assert.Contains(t, f.bodies[0], `"openConversationId":"cid-001"`)
	assert.Contains(t, f.bodies[0], `"robotCode":"app-key"`)
}

// TestChannel_ServeHTTP_SignatureAndNormalize covers the incoming path:
// bad signature 401, good signature forwards a normalized ChannelEvent,
// non-text msgtypes are ACKed and dropped.
func TestChannel_ServeHTTP_SignatureAndNormalize(t *testing.T) {
	ch := New("dt-1", "app-key", "app-secret").WithRobotSecret("robot-secret")
	var got []channel.ChannelEvent
	ch.WithEmitter(func(ev channel.ChannelEvent) error {
		got = append(got, ev)
		return nil
	})

	post := func(body string, sign, ts bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if sign {
			tsStr, sig := SignOutgoing("robot-secret", time.Now())
			req.Header.Set("timestamp", tsStr)
			req.Header.Set("sign", sig)
		}
		if ts {
			req.Header.Set("timestamp", "123")
		}
		w := httptest.NewRecorder()
		ch.ServeHTTP(w, req)
		return w
	}

	// No signature headers -> 401.
	w := post(`{}`, false, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, got)

	// Valid signature + text message -> forwarded and normalized.
	w = post(`{"conversationId":"cid-9","conversationTitle":"ops","senderStaffId":"u1","senderNick":"Alice","msgId":"m1","msgtype":"text","text":{"content":"deploy now"},"sessionWebhook":"https://oapi/callback"}`, true, false)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, got, 1)
	ev := got[0]
	assert.Equal(t, "cid-9", ev.ChatID)
	assert.Equal(t, "u1", ev.ChannelUserID)
	assert.Equal(t, "Alice", ev.ChannelUserName)
	assert.Equal(t, "deploy now", ev.Text)
	assert.Equal(t, "dt-1", ev.ChannelID)
	assert.Equal(t, "https://oapi/callback", ev.Metadata["session_webhook"])

	// Non-text -> ACK, no event.
	w = post(`{"msgtype":"picture"}`, true, false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, got, 1)
}

// TestConfirmDecisionFromCallback verifies the card-callback → HITL mapping.
func TestConfirmDecisionFromCallback(t *testing.T) {
	h := func(action string) *CardCallback {
		return &CardCallback{TrackID: "turn-1", UserID: "u1", ActionValue: []byte(action)}
	}

	// A HITL decision action maps onto the resume event.
	ev, err := ConfirmDecisionFromCallback(h(`{"type":"hitl_decision","session_id":"s1","reply_id":"r1","confirm_id":"c1","decisions":[{"tool_call_id":"tc1","decision":"allow"}]}`))
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "c1", ev.ConfirmID)
	require.Len(t, ev.Decisions, 1)
	assert.Equal(t, "allow", ev.Decisions[0].Decision)

	// Non-HITL actions are explicitly rejected so callers can route them.
	_, err = ConfirmDecisionFromCallback(h(`{"type":"custom_action","foo":1}`))
	assert.ErrorIs(t, err, ErrNotHITLAction)
	_, err = ConfirmDecisionFromCallback(h(``))
	assert.ErrorIs(t, err, ErrNotHITLAction)
}
