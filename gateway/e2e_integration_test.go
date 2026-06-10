package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

// e2eAuthAgent implements V2Agent and signals completion via a channel.
type e2eAuthAgent struct {
	done chan struct{}
}

func newE2EAuthAgent() *e2eAuthAgent { return &e2eAuthAgent{done: make(chan struct{})} }

func (a *e2eAuthAgent) Name() string { return "e2e-auth" }
func (a *e2eAuthAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}
func (a *e2eAuthAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}
func (a *e2eAuthAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return a.Call(ctx, msg)
}
func (a *e2eAuthAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 4)
	go func() {
		defer close(ch)
		defer close(a.done)
		ch <- event.NewTextBlockDelta("r1", 0, "hello")
		ch <- event.NewTextBlockDelta("r1", 0, " from auth")
		ch <- event.NewReplyEnd("r1", "")
	}()
	return ch, nil
}
func (a *e2eAuthAgent) LoadState(state *agent.AgentState) error { return nil }
func (a *e2eAuthAgent) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (a *e2eAuthAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error { return nil }

var _ agent.V2Agent = (*e2eAuthAgent)(nil)

// ================================================================
// E2E Tests
// ================================================================

// TestE2E_FullAuthFlow verifies register → login → use JWT on V2 route.
func TestE2E_FullAuthFlow(t *testing.T) {
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte("test-secret"), "test-issuer")

	// Step 1: Register a new user.
	srv := NewServer(&mockV2Agent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	regBody := `{"name":"Alice","email":"alice@test.com"}`
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(regBody))
	regRec := httptest.NewRecorder()
	srv.ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d body=%s", regRec.Code, regRec.Body.String())
	}

	var regResp registerResponse
	if err := json.Unmarshal(regRec.Body.Bytes(), &regResp); err != nil {
		t.Fatal(err)
	}
	if regResp.UserID == "" {
		t.Fatal("register: expected user_id")
	}

	// Step 2: Login to get JWT token.
	loginBody, _ := json.Marshal(map[string]string{"user_id": regResp.UserID})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	srv.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp loginResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatal(err)
	}
	if loginResp.Token == "" {
		t.Fatal("login: expected token")
	}

	// Step 3: Use JWT to access a V2 route.
	srv.WithAuthenticator(jwtAuth)
	srv.RegisterV2Routes()

	chatBody, _ := json.Marshal(chatRequest{Text: "hello"})
	chatReq := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(chatBody))
	chatReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	chatReq.Header.Set("Content-Type", "application/json")
	chatRec := httptest.NewRecorder()
	srv.ServeHTTP(chatRec, chatReq)

	if chatRec.Code != http.StatusOK {
		t.Fatalf("chat: expected 200, got %d body=%s", chatRec.Code, chatRec.Body.String())
	}
	if !strings.Contains(chatRec.Body.String(), "hello") {
		t.Fatal("chat: expected 'hello' in SSE stream")
	}

	// Step 4: Verify JWT rejection without token.
	unauthReq := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(chatBody))
	unauthRec := httptest.NewRecorder()
	srv.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: expected 401, got %d", unauthRec.Code)
	}
}

// TestE2E_SSE_WithAuthAndSessionID verifies V2 SSE with auth and session tracking.
func TestE2E_SSE_WithAuthAndSessionID(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")

	ctx := context.Background()
	user := &service.User{ID: "u-sse", Name: "sse-user"}
	storage.SaveUser(ctx, user)
	cred := &service.Credential{ID: "c-tok", UserID: "u-sse", Provider: "api_key", Label: "sse", Encrypted: "sse-key"}
	storage.SaveCredential(ctx, cred)

	sm := NewSessionManager()
	srv := NewServer(&mockV2Agent{}).
		WithStorage(storage).
		WithAuthenticator(apiAuth).
		WithSessionManager(sm)
	srv.RegisterV2Routes()

	body, _ := json.Marshal(v2ChatRequest{Text: "hello", SessionID: "sess-sse"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "sse-key")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "hello") {
		t.Fatal("expected 'hello' in SSE stream")
	}
	if !strings.Contains(bodyStr, "done") {
		t.Fatal("expected 'done' in SSE stream")
	}
}

// TestE2E_StreamableHTTP_FullLifecycle demonstrates the full Streamable HTTP flow.
func TestE2E_StreamableHTTP_FullLifecycle(t *testing.T) {
	srv := NewServer(&slowMockV2Agent{delay: 2 * time.Second}).
		WithSessionManager(NewSessionManager())
	srv.RegisterV2Routes()

	accept := streamableAcceptHeader()

	// Step 1: POST to start a run.
	startBody, _ := json.Marshal(v2ChatRequest{Text: "hi", SessionID: "sh-sess"})
	startReq := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(startBody))
	startReq.Header.Set("Accept", accept)
	startRec := httptest.NewRecorder()

	go srv.ServeHTTP(startRec, startReq)
	time.Sleep(50 * time.Millisecond)

	// Step 2: DELETE to terminate while run is still active.
	delReq := httptest.NewRequest(http.MethodDelete, "/v2/chat?session_id=sh-sess", nil)
	delRec := httptest.NewRecorder()
	srv.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d body=%s", delRec.Code, delRec.Body.String())
	}

	// Step 3: DELETE again — should be not found.
	delReq2 := httptest.NewRequest(http.MethodDelete, "/v2/chat?session_id=sh-sess", nil)
	delRec2 := httptest.NewRecorder()
	srv.ServeHTTP(delRec2, delReq2)
	if delRec2.Code != http.StatusNotFound {
		t.Fatalf("DELETE not found: expected 404, got %d", delRec2.Code)
	}

	// Step 4: Start a new run and GET subscribe to verify replay.
	startBody2, _ := json.Marshal(v2ChatRequest{Text: "hi", SessionID: "sh-sess2"})
	startReq2 := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(startBody2))
	startReq2.Header.Set("Accept", accept)
	startRec2 := httptest.NewRecorder()

	go srv.ServeHTTP(startRec2, startReq2)
	time.Sleep(50 * time.Millisecond)

	getReq := httptest.NewRequest(http.MethodGet, "/v2/chat?session_id=sh-sess2", nil)
	getReq.Header.Set("Accept", accept)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET subscribe: expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	if sid := getRec.Header().Get(HeaderAgentSessionID); sid != "sh-sess2" {
		t.Fatalf("expected Agent-Session-Id sh-sess2, got %q", sid)
	}
	if !strings.Contains(getRec.Body.String(), "hello") {
		t.Fatalf("GET subscribe should replay events:\n%s", getRec.Body.String())
	}
}

// TestE2E_AGUI_ProtocolWithContentNegotiation verifies AG-UI with auth.
func TestE2E_AGUI_ProtocolWithContentNegotiation(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")

	// Pre-register a user for auth.
	ctx := context.Background()
	user := &service.User{ID: "u-agui", Name: "tester"}
	storage.SaveUser(ctx, user)
	cred := &service.Credential{
		ID:       "c-agui",
		UserID:   "u-agui",
		Provider: "api_key",
		Label:    "default",
		Encrypted: "test-api-key-xyz",
	}
	storage.SaveCredential(ctx, cred)

	srv := NewServer(&mockV2Agent{}).
		WithStorage(storage).
		WithAuthenticator(apiAuth)
	srv.RegisterV2Routes()

	// Send authenticated AG-UI request.
	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream?protocol=agui", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key-xyz")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("AG-UI with auth: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "TEXT_MESSAGE_CONTENT") {
		t.Fatalf("expected AG-UI TEXT_MESSAGE_CONTENT, got:\n%s", respBody)
	}
	if !strings.Contains(respBody, "STREAM_DONE") {
		t.Fatalf("expected AG-UI STREAM_DONE, got:\n%s", respBody)
	}
}

// TestE2E_V2Chat_RateLimit_InvalidAuth verifies auth edge cases.
func TestE2E_V2Chat_RateLimit_InvalidAuth(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")

	srv := NewServer(&mockV2Agent{}).
		WithStorage(storage).
		WithAuthenticator(apiAuth)
	srv.RegisterV2Routes()

	tests := []struct {
		name   string
		header string
		key    string
		want   int
	}{
		{"no header", "", "", http.StatusUnauthorized},
		{"wrong key", "X-API-Key", "wrong-key", http.StatusUnauthorized},
		{"empty key", "X-API-Key", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(chatRequest{Text: "hi"})
			req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(body))
			if tt.header != "" || tt.key != "" {
				req.Header.Set(tt.header, tt.key)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("%s: expected %d, got %d", tt.name, tt.want, rec.Code)
			}
		})
	}
}

// TestE2E_MultipleSessionsIsolation verifies session isolation with slow agents.
func TestE2E_MultipleSessionsIsolation(t *testing.T) {
	sm := NewSessionManager()
	slow := &slowMockV2Agent{delay: 200 * time.Millisecond}
	srv := NewServer(slow).
		WithSessionManager(sm)
	srv.RegisterV2Routes()

	accept := streamableAcceptHeader()

	// Start session A.
	aBody, _ := json.Marshal(v2ChatRequest{Text: "hi", SessionID: "sess-a"})
	aReq := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(aBody))
	aReq.Header.Set("Accept", accept)
	aRec := httptest.NewRecorder()
	go srv.ServeHTTP(aRec, aReq)
	time.Sleep(20 * time.Millisecond)

	// Start session B.
	bBody, _ := json.Marshal(v2ChatRequest{Text: "hi", SessionID: "sess-b"})
	bReq := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(bBody))
	bReq.Header.Set("Accept", accept)
	bRec := httptest.NewRecorder()
	go srv.ServeHTTP(bRec, bReq)
	time.Sleep(20 * time.Millisecond)

	if !sm.IsActive("sess-a") {
		t.Fatal("sess-a should be active")
	}
	if !sm.IsActive("sess-b") {
		t.Fatal("sess-b should be active")
	}
	if sm.ActiveCount() != 2 {
		t.Fatalf("expected 2 active sessions, got %d", sm.ActiveCount())
	}

	// Terminate session A.
	sm.Terminate("sess-a")

	// Wait for termination.
	for i := 0; i < 50 && sm.IsActive("sess-a"); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if sm.IsActive("sess-a") {
		t.Fatal("sess-a should be terminated")
	}
	if !sm.IsActive("sess-b") {
		t.Fatal("sess-b should still be active")
	}
}

// TestE2E_V2ChatStreamable_ContentNegotiation verifies content-type negotiation.
func TestE2E_V2ChatStreamable_ContentNegotiation(t *testing.T) {
	srv := NewServer(&slowMockV2Agent{delay: 20 * time.Millisecond})
	srv.RegisterV2Routes()

	accept := streamableAcceptHeader()

	// POST with proper Accept → SSE.
	body, _ := json.Marshal(v2ChatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(body))
	req.Header.Set("Accept", accept)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

	// POST without Accept → 406.
	req2 := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d", rec2.Code)
	}
}
