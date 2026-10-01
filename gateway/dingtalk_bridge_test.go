package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/channel"
	"github.com/linkerlin/agentscope.go/channel/dingtalk"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/service"
)

// dingtalkCredentialValue is the 18.4 authorize payload shape for a DingTalk
// enterprise app.
func dingtalkCredentialValue(appKey, appSecret, robotSecret string) string {
	b, _ := json.Marshal(map[string]string{
		"app_key": appKey, "app_secret": appSecret, "robot_secret": robotSecret,
	})
	return string(b)
}

// TestResolveDingtalkCredential locks the 18.4 gate: PENDING bindings are
// refused (an unfinished binding must never yield platform credentials),
// AUTHORIZED ones decrypt through the cipher, and unknown ids 404-style
// error out.
func TestResolveDingtalkCredential(t *testing.T) {
	ctx := context.Background()
	storage := service.NewMemoryStorage()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte('a' + i)
	}
	cipher, err := service.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	enc, err := cipher.Encrypt(dingtalkCredentialValue("dk-key", "dk-secret", "dk-robot"))
	if err != nil {
		t.Fatal(err)
	}
	// PENDING binding: refused.
	_ = storage.SaveCredential(ctx, &service.Credential{
		ID: "cred-pending", UserID: "u1", Provider: "dingtalk",
		Encrypted: enc, Status: service.CredentialPending, BindingRef: "flow-1",
	})
	if _, _, _, err := ResolveDingtalkCredential(ctx, storage, cipher, "cred-pending"); err == nil {
		t.Fatal("PENDING credential must not resolve to platform credentials")
	} else if !strings.Contains(err.Error(), "not usable") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Authorize → resolves, secrets decrypt correctly.
	if _, err := service.TransitionCredential(ctx, storage, "cred-pending", service.CredentialAuthorized, nil); err != nil {
		t.Fatal(err)
	}
	appKey, appSecret, robotSecret, err := ResolveDingtalkCredential(ctx, storage, cipher, "cred-pending")
	if err != nil {
		t.Fatal(err)
	}
	if appKey != "dk-key" || appSecret != "dk-secret" || robotSecret != "dk-robot" {
		t.Fatalf("decrypted payload wrong: %q %q %q", appKey, appSecret, robotSecret)
	}

	// Unknown id.
	if _, _, _, err := ResolveDingtalkCredential(ctx, storage, cipher, "cred-nope"); err == nil {
		t.Fatal("unknown credential must not resolve")
	}

	// End-to-end: the channel built from the credential verifies signatures
	// with the decrypted robot secret.
	ch, err := NewDingtalkChannelFromCredential(ctx, "dk1", "cred-pending", storage, cipher)
	if err != nil {
		t.Fatal(err)
	}
	ts, sign := dingtalk.SignOutgoing("dk-robot", time.Now())
	req := httptest.NewRequest(http.MethodPost, "/cb", nil)
	req.Header.Set("timestamp", ts)
	req.Header.Set("sign", sign)
	if !ch.VerifyRequest(req) {
		t.Fatal("channel built from 18.4 credential failed signature verification")
	}
}

// hitlCallbackBody builds a signed card callback carrying a HITL decision.
func hitlCallbackBody(t *testing.T, sessionID, confirmID string) (*http.Request, *dingtalk.Channel) {
	t.Helper()
	ch := dingtalk.New("dk1", "k", "s").WithRobotSecret("robot-secret")
	action := map[string]any{
		"type":       "hitl_decision",
		"session_id": sessionID,
		"reply_id":   "reply-1",
		"confirm_id": confirmID,
		"decisions":  []map[string]string{{"tool_call_id": "tc1", "decision": "allow"}},
	}
	val, _ := json.Marshal(action)
	body, _ := json.Marshal(map[string]any{
		"cardId":  "card-1",
		"trackId": "track-1",
		"userId":  "operator-1",
		"content": map[string]any{"value": json.RawMessage(val)},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/dk1/dingtalk/card-callback", strings.NewReader(string(body)))
	ts, sign := dingtalk.SignOutgoing("robot-secret", time.Now())
	req.Header.Set("timestamp", ts)
	req.Header.Set("sign", sign)
	req.Header.Set("Content-Type", "application/json")
	return req, ch
}

// suspendedResumeAgent records InjectEvent calls; the callback bridge must
// deliver exactly one decision regardless of duplicate callbacks.
type suspendedResumeAgent struct {
	fakeV2Agent
	injectCalls int
}

func (a *suspendedResumeAgent) LoadState(s *agent.AgentState) error { return nil }
func (a *suspendedResumeAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	a.injectCalls++
	return nil
}

// TestDingtalkCardCallbackIdempotentHITL walks the closed loop: signed
// callback → HITL action → 23.2 resume; a duplicate callback is absorbed
// idempotently (200, no re-delivery); a bad signature is 401; an unknown
// session is 404.
func TestDingtalkCardCallbackIdempotentHITL(t *testing.T) {
	f := newSessionIdentityFixture(t)
	srv := f.srv
	_ = srv.storage.SaveAgentConfig(context.Background(), &service.AgentConfig{ID: "wa", UserID: "u1", Name: "W", Source: "team"})
	_ = srv.storage.SaveSession(context.Background(), &service.Session{ID: "hitl-s", UserID: "u1", AgentID: "wa"})

	stub := &suspendedResumeAgent{}
	// Wire a dingtalk channel + the callback route.
	ch := dingtalk.New("dk1", "k", "s").WithRobotSecret("robot-secret")
	reg := channel.NewRegistry()
	reg.Register(ch)
	srv.WithChannelGateway(reg, channel.NewGateway(nil, nil))
	srv.RegisterChannelRoutes()

	// Suspended snapshot so the resume state machine takes the persisted path.
	suspended := time.Now().UTC()
	_ = srv.storage.SaveSnapshot(context.Background(), &service.AgentSnapshot{
		SessionID: "hitl-s",
		ReplyID:   "reply-1",
		State:     &agent.AgentState{Version: "v2", ReplyID: "reply-1", SuspendedAt: &suspended},
	})

	// Bridge agent resolution is injectable; the test stubs it.
	srv.dingtalkAgentResolver = func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		if agentID == "wa" {
			return stub, nil
		}
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	req, _ := hitlCallbackBody(t, "hitl-s", "cf-1")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first callback: %d body=%s", rec.Code, rec.Body.String())
	}

	// Duplicate callback (double click / platform retry): 200, idempotent.
	req2, _ := hitlCallbackBody(t, "hitl-s", "cf-1")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("duplicate callback: %d body=%s", rec2.Code, rec2.Body.String())
	}

	// Bad signature: 401.
	req3, _ := hitlCallbackBody(t, "hitl-s", "cf-2")
	req3.Header.Set("sign", "forged")
	rec3 := httptest.NewRecorder()
	srv.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Fatalf("forged signature: expected 401, got %d", rec3.Code)
	}

	// Unknown session: 404.
	req4, _ := hitlCallbackBody(t, "nope-s", "cf-3")
	rec4 := httptest.NewRecorder()
	srv.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusNotFound {
		t.Fatalf("unknown session: expected 404, got %d", rec4.Code)
	}

	// Non-HITL action: 200 ACK (custom button).
	custom, _ := json.Marshal(map[string]any{"type": "custom_action"})
	body2, _ := json.Marshal(map[string]any{"content": map[string]any{"value": json.RawMessage(custom)}})
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/channels/dk1/dingtalk/card-callback", strings.NewReader(string(body2)))
	ts, sign := dingtalk.SignOutgoing("robot-secret", time.Now())
	req5.Header.Set("timestamp", ts)
	req5.Header.Set("sign", sign)
	rec5 := httptest.NewRecorder()
	srv.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusOK {
		t.Fatalf("non-HITL action: expected 200 ACK, got %d", rec5.Code)
	}
}
