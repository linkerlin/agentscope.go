package gateway

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/service"
)

// tenantEnv builds a V2 server with storage and two registered tenants.
func tenantEnv(t *testing.T) (*Server, *service.MemoryStorage, map[string]string) {
	t.Helper()
	storage := service.NewMemoryStorage()
	ctx := context.Background()

	keys := make(map[string]string)
	for _, uid := range []string{"u-tenant-a", "u-tenant-b"} {
		storage.SaveUser(ctx, &service.User{ID: uid, Name: uid})
		key, err := service.GenerateAPIKey()
		if err != nil {
			t.Fatal(err)
		}
		storage.SaveCredential(ctx, &service.Credential{
			ID: "cred-" + uid, UserID: uid, Provider: "api_key",
			Encrypted: service.HashAPIKey(key),
		})
		keys[uid] = key
	}

	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockV2Agent{}).
		WithStorage(storage).
		WithAuthenticator(apiAuth).
		WithSessionManager(NewSessionManager())
	srv.RegisterV2Routes()
	return srv, storage, keys
}

func tenantPost(t *testing.T, srv *Server, path, apiKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// TestV2Chat_UnknownSessionRefused locks the 22.2 rule: with storage
// configured, a session ID that is not persisted is refused — a client
// cannot claim an arbitrary ID by merely sending it.
func TestV2Chat_UnknownSessionRefused(t *testing.T) {
	srv, _, keys := tenantEnv(t)

	rec := tenantPost(t, srv, "/v2/chat/stream", keys["u-tenant-a"],
		`{"text":"hi","session_id":"sess-forged"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown session, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestV2Chat_ServerMintsSessionID locks the 22.2 rule: a request without a
// session ID gets a server-minted one back via the session header, persisted
// with the caller's ownership.
func TestV2Chat_ServerMintsSessionID(t *testing.T) {
	srv, storage, keys := tenantEnv(t)

	rec := tenantPost(t, srv, "/v2/chat/stream", keys["u-tenant-a"], `{"text":"hi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	minted := rec.Header().Get(HeaderAgentSessionID)
	if minted == "" || strings.Contains(minted, "UnixNano") {
		t.Fatalf("expected server-minted session id header, got %q", minted)
	}
	se, err := storage.GetSession(context.Background(), minted)
	if err != nil {
		t.Fatalf("minted session not persisted: %v", err)
	}
	if se.UserID != "u-tenant-a" {
		t.Fatalf("minted session owned by %q, want u-tenant-a", se.UserID)
	}

	// The minted ID is usable in a follow-up request by its owner.
	rec = tenantPost(t, srv, "/v2/chat/stream", keys["u-tenant-a"],
		`{"text":"again","session_id":"`+minted+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("follow-up with minted id: %d %s", rec.Code, rec.Body.String())
	}
}

// TestV2Chat_CrossTenantSameExternalID locks the 22.2 rule: two tenants
// referencing the same session ID — the owner succeeds, the other tenant
// gets 404 regardless of HTTP method (POST / DELETE / GET).
func TestV2Chat_CrossTenantSameExternalID(t *testing.T) {
	srv, storage, keys := tenantEnv(t)
	ctx := context.Background()

	// Tenant A starts a session (server-minted).
	rec := tenantPost(t, srv, "/v2/chat/stream", keys["u-tenant-a"], `{"text":"hi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant A chat: %d %s", rec.Code, rec.Body.String())
	}
	sessID := rec.Header().Get(HeaderAgentSessionID)
	if sessID == "" {
		t.Fatal("expected minted session header")
	}

	// A second session owned by tenant B with the same external ID must not
	// be possible to forge: B referencing A's ID is refused everywhere.
	for _, tc := range []struct {
		name, method, path string
	}{
		{"post", http.MethodPost, "/v2/chat/stream"},
		{"delete", http.MethodDelete, "/v2/chat?session_id=" + sessID},
		{"get", http.MethodGet, "/v2/chat?session_id=" + sessID},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("X-API-Key", keys["u-tenant-b"])
		if tc.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
			req.Body = io.NopCloser(bytes.NewReader([]byte(`{"text":"hi","session_id":"` + sessID + `"}`)))
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: tenant B using tenant A's session: expected 404, got %d: %s",
				tc.name, rec.Code, rec.Body.String())
		}
	}

	// Owner still succeeds.
	rec = tenantPost(t, srv, "/v2/chat/stream", keys["u-tenant-a"],
		`{"text":"ok","session_id":"`+sessID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner reuse: %d %s", rec.Code, rec.Body.String())
	}

	// Cross-tenant ownership record sanity.
	se, _ := storage.GetSession(ctx, sessID)
	if se.UserID != "u-tenant-a" {
		t.Fatalf("session ownership changed: %q", se.UserID)
	}
}

// TestV2Chat_WSKnownSessionOnly locks the 22.2 rule on the WebSocket
// handshake: with storage configured, an unpersisted session ID is refused
// before the upgrade.
func TestV2Chat_WSKnownSessionOnly(t *testing.T) {
	srv, _, keys := tenantEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/v2/chat/ws?session=sess-forged", nil)
	req.Header.Set("X-API-Key", keys["u-tenant-a"])
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before upgrade for unknown session, got %d", rec.Code)
	}
}

// TestV2Chat_ResumeUnknownSessionRefused locks the 22.2 rule on the resume
// endpoint: unknown session IDs are refused, not treated as fresh sessions.
func TestV2Chat_ResumeUnknownSessionRefused(t *testing.T) {
	srv, _, keys := tenantEnv(t)

	rec := tenantPost(t, srv, "/v2/resume", keys["u-tenant-a"],
		`{"session_id":"sess-forged","event":{}}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for resume of unknown session, got %d: %s", rec.Code, rec.Body.String())
	}
}
