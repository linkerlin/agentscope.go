package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/service"
)

// sessionIdentityFixture builds a storage+auth server exactly the way the
// web_ui production assembly does (18.10): MemoryStorage, JWT, V2 routes.
type sessionIdentityFixture struct {
	srv   *Server
	token string
}

func newSessionIdentityFixture(t *testing.T) *sessionIdentityFixture {
	t.Helper()
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte("test-secret"), "test-issuer")
	srv := NewServer(&mockV2Agent{}).
		WithStorage(storage).
		WithSessionManager(NewSessionManager().WithStorage(storage)).
		WithAuthenticator(jwtAuth)
	srv.RegisterAuthRoutes(jwtAuth)
	srv.RegisterV2Routes()

	// Register + login → bearer token (22.1 proof-of-possession flow).
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"name":"Alice"}`))
	regRec := httptest.NewRecorder()
	srv.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: %d body=%s", regRec.Code, regRec.Body.String())
	}
	var regResp registerResponse
	if err := json.Unmarshal(regRec.Body.Bytes(), &regResp); err != nil {
		t.Fatal(err)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"api_key":"`+regResp.APIKey+`"}`))
	loginRec := httptest.NewRecorder()
	srv.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	var loginResp loginResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatal(err)
	}
	return &sessionIdentityFixture{srv: srv, token: loginResp.Token}
}

func (f *sessionIdentityFixture) post(t *testing.T, path, body string, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set(HeaderAgentSessionID, sessionID)
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, req)
	return rec
}

// TestWebUI_SessionIdentity_ServerMinted locks the 18.10 acceptance flow
// against a storage+auth server: the console's first packet omits session_id
// entirely, the server mints one and returns it in Agent-Session-Id, and the
// adopted ID works for steering and interrupting. A client-fabricated UUID
// (the pre-18.10 behaviour) is refused 404.
func TestWebUI_SessionIdentity_ServerMinted(t *testing.T) {
	f := newSessionIdentityFixture(t)
	ctx := context.Background()

	// 1. Fabricated UUID (what app.js used to localStorage-mint): unknown
	// session under storage → 404, no existence leak.
	fabricated := "11111111-2222-3333-4444-555555555555"
	rec := f.post(t, "/v2/chat", `{"text":"hi"}`, fabricated)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("fabricated session: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 2. First packet without session_id: server mints and returns it.
	rec2 := f.post(t, "/v2/chat", `{"text":"hi"}`, "")
	if rec2.Code != http.StatusOK {
		t.Fatalf("first packet: expected 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	minted := rec2.Header().Get(HeaderAgentSessionID)
	if minted == "" || minted == fabricated {
		t.Fatalf("server did not mint a fresh session id: %q", minted)
	}
	// Ownership persisted (22.2): the session record exists under the caller.
	if _, err := f.srv.storage.GetSession(ctx, minted); err != nil {
		t.Fatalf("minted session not persisted: %v", err)
	}

	// 3. Adopted ID: subsequent turn with the server ID is 2xx.
	rec3 := f.post(t, "/v2/chat", `{"text":"again"}`, minted)
	if rec3.Code != http.StatusOK {
		t.Fatalf("adopted session turn: expected 200, got %d body=%s", rec3.Code, rec3.Body.String())
	}
	if got := rec3.Header().Get(HeaderAgentSessionID); got != minted {
		t.Fatalf("adopted id echoed wrong: %q", got)
	}

	// 4. Steer with the adopted ID: ownership passes (the session exists
	// under this caller), so the response is about run state — 409 (no
	// active run) or any 2xx — never the ownership 404 "session not found".
	rec4 := f.post(t, "/v2/sessions/"+minted+"/steer", `{"text":"go left"}`, "")
	if rec4.Code == http.StatusNotFound && strings.Contains(rec4.Body.String(), "session not found") {
		t.Fatalf("steer with adopted id rejected on ownership grounds: %s", rec4.Body.String())
	}

	// 5. Interrupt with the adopted ID: same identity distinction — the
	// 404 body names the run state, not ownership.
	rec5 := f.post(t, "/v2/sessions/"+minted+"/interrupt", ``, "")
	if rec5.Code == http.StatusNotFound && strings.Contains(rec5.Body.String(), "session not found") {
		t.Fatalf("interrupt with adopted id rejected on ownership grounds: %s", rec5.Body.String())
	}

	// 6. Steer/interrupt with a foreign session id stays refused (22.2).
	rec6 := f.post(t, "/v2/sessions/"+fabricated+"/steer", `{"text":"x"}`, "")
	if rec6.Code != http.StatusNotFound {
		t.Fatalf("steer with fabricated id: expected 404, got %d", rec6.Code)
	}

	_ = time.Now
}
