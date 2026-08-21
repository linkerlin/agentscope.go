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

	"github.com/gorilla/websocket"

	"github.com/linkerlin/agentscope.go/service"
)

// tenantRequest builds an HTTP request with a simulated authenticated user id
// (as if requireAuth had resolved it).
func tenantRequest(t *testing.T, method, path string, body any, userID string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		requireNoErr(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(context.WithValue(req.Context(), service.ContextKeyUserID, userID))
	return req
}

func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// newMultiTenantEnv builds a gateway with two persisted sessions owned by two
// different users (s-a -> u1, s-b -> u2) and a slow agent so runs stay active.
// u1/u2 are the same server wrapped with a simulated authenticated identity.
func newMultiTenantEnv(t *testing.T) (*Server, *SessionManager, http.Handler, http.Handler) {
	t.Helper()
	storage := service.NewMemoryStorage()
	now := time.Now()
	for _, s := range []*service.Session{
		{ID: "s-a", UserID: "u1", AgentID: "a1", CreatedAt: now, UpdatedAt: now},
		{ID: "s-b", UserID: "u2", AgentID: "a1", CreatedAt: now, UpdatedAt: now},
	} {
		requireNoErr(t, storage.SaveSession(context.Background(), s))
	}

	sm := NewSessionManager().WithStorage(storage)
	srv := NewServer(&slowMockV2Agent{delay: 300 * time.Millisecond}).
		WithStorage(storage).
		WithSessionManager(sm)
	srv.RegisterV2Routes()

	asUser := func(userID string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), service.ContextKeyUserID, userID))
			srv.ServeHTTP(w, r)
		})
	}
	return srv, sm, asUser("u1"), asUser("u2")
}

// TestMultiTenant_StreamableHTTPSessionIsolation verifies that a user cannot
// POST/GET/DELETE another user's session over the Streamable HTTP transport.
func TestMultiTenant_StreamableHTTPSessionIsolation(t *testing.T) {
	srv, sm, _, _ := newMultiTenantEnv(t)
	accept := streamableAcceptHeader()

	// u2 starts her own run on s-b (long-lived SSE stream in a goroutine).
	startDone := make(chan struct{})
	go func() {
		defer close(startDone)
		req := tenantRequest(t, "POST", "/v2/chat",
			map[string]any{"text": "hi", "session_id": "s-b"}, "u2")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !sm.IsActive("s-b") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !sm.IsActive("s-b") {
		t.Fatal("u2's run did not become active")
	}

	// u1 touching u2's active session is rejected on all three verbs.
	for _, c := range []struct{ method, path string }{
		{"POST", "/v2/chat"},
		{"GET", "/v2/chat?session_id=s-b"},
		{"DELETE", "/v2/chat?session_id=s-b"},
	} {
		var body any
		if c.method == "POST" {
			body = map[string]any{"text": "hi", "session_id": "s-b"}
		}
		req := tenantRequest(t, c.method, c.path, body, "u1")
		if c.method == "POST" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("u1 %s %s: expected 404, got %d", c.method, c.path, rec.Code)
		}
	}

	// u1 reading her own session still works.
	req := tenantRequest(t, "GET", "/v2/chat?session_id=s-a", nil, "u1")
	req.Header.Set("Accept", accept)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("u1 own session GET: expected 200, got %d", rec.Code)
	}

	// u2 can terminate her own run.
	req = tenantRequest(t, "DELETE", "/v2/chat?session_id=s-b", nil, "u2")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("u2 own DELETE: expected 204, got %d", rec.Code)
	}
	<-startDone
}

// TestMultiTenant_WSSessionIsolation verifies WebSocket handshake rejection
// when a user dials another user's session.
func TestMultiTenant_WSSessionIsolation(t *testing.T) {
	_, _, u1, u2 := newMultiTenantEnv(t)
	srv1 := httptest.NewServer(u1)
	defer srv1.Close()
	srv2 := httptest.NewServer(u2)
	defer srv2.Close()

	wsURL := func(srv *httptest.Server, session string) string {
		return "ws" + strings.TrimPrefix(srv.URL, "http") + "/v2/chat/ws?session=" + session
	}

	// u2 dials her own session: handshake succeeds.
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv2, "s-b"), nil)
	if err != nil {
		t.Fatalf("u2 dial own session failed: %v (status %d)", err, resp.StatusCode)
	}
	conn.Close()

	// u1 dials u2's session: handshake is rejected with 404.
	conn, resp, err = websocket.DefaultDialer.Dial(wsURL(srv1, "s-b"), nil)
	if err == nil {
		conn.Close()
		t.Fatal("u1 should not be able to dial u2's session")
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 handshake rejection, got %+v", resp)
	}

	// u1 dials her own session: succeeds.
	conn, resp, err = websocket.DefaultDialer.Dial(wsURL(srv1, "s-a"), nil)
	if err != nil {
		t.Fatalf("u1 dial own session failed: %v (status %d)", err, resp.StatusCode)
	}
	conn.Close()
}
