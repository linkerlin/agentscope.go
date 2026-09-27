package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/service"
)

// newTestAPIKeyUser registers a user holding an api_key credential stored in
// hashed form and returns the plaintext key (generated at runtime, never
// hardcoded — 22.1).
func newTestAPIKeyUser(t *testing.T, storage *service.MemoryStorage, userID string) string {
	t.Helper()
	ctx := context.Background()
	storage.SaveUser(ctx, &service.User{ID: userID, Name: "Alice"})
	key, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	storage.SaveCredential(ctx, &service.Credential{
		ID: "cred-" + userID, UserID: userID, Provider: "api_key",
		Encrypted: service.HashAPIKey(key),
	})
	return key
}

func postJSON(t *testing.T, srv *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestHandleRegister(t *testing.T) {
	storage := service.NewMemoryStorage()
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(nil)

	rec := postJSON(t, srv, "/api/v1/auth/register", `{"name":"Alice"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp registerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.UserID == "" {
		t.Fatal("expected user_id")
	}
	if resp.APIKey == "" {
		t.Fatal("expected api_key")
	}
	// The stored credential must hold a digest, not the returned key (22.1).
	creds, err := storage.ListCredentialsByUser(context.Background(), resp.UserID)
	if err != nil || len(creds) != 1 {
		t.Fatalf("expected one stored credential, got %v (%v)", len(creds), err)
	}
	if !service.IsHashedAPIKey(creds[0].Encrypted) {
		t.Fatal("stored credential is not a hashed API key")
	}
	if creds[0].Encrypted == resp.APIKey {
		t.Fatal("stored credential leaks the plaintext key")
	}
}

func TestHandleRegisterMissingName(t *testing.T) {
	storage := service.NewMemoryStorage()
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(nil)

	rec := postJSON(t, srv, "/api/v1/auth/register", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleLogin(t *testing.T) {
	storage := service.NewMemoryStorage()
	key := newTestAPIKeyUser(t, storage, "u1")

	jwtAuth := service.NewJWTAuthenticator([]byte("unit-test-signing-secret"), "test")
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	rec := postJSON(t, srv, "/api/v1/auth/login", fmt.Sprintf(`{"api_key":%q}`, key))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" {
		t.Fatal("expected token")
	}
}

func TestHandleLoginInvalidKey(t *testing.T) {
	storage := service.NewMemoryStorage()
	_ = newTestAPIKeyUser(t, storage, "u1")
	otherKey, err := service.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}

	jwtAuth := service.NewJWTAuthenticator([]byte("unit-test-signing-secret"), "test")
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	rec := postJSON(t, srv, "/api/v1/auth/login", fmt.Sprintf(`{"api_key":%q}`, otherKey))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// TestHandleLoginUserIDAloneRejected locks the 22.1 rule: knowing a user ID
// is not a credential — the old user_id-only login must not mint tokens.
func TestHandleLoginUserIDAloneRejected(t *testing.T) {
	storage := service.NewMemoryStorage()
	_ = newTestAPIKeyUser(t, storage, "u1")

	jwtAuth := service.NewJWTAuthenticator([]byte("unit-test-signing-secret"), "test")
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	rec := postJSON(t, srv, "/api/v1/auth/login", `{"user_id":"u1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (api_key required), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleLoginLegacyPlaintextRejected locks the 22.1 rule: credentials
// stored in legacy plaintext form (the key itself instead of its digest)
// never authenticate.
func TestHandleLoginLegacyPlaintextRejected(t *testing.T) {
	storage := service.NewMemoryStorage()
	key := newTestAPIKeyUser(t, storage, "u1")
	// Downgrade the stored form to legacy plaintext to simulate an
	// un-migrated credential, then prove it no longer authenticates.
	ctx := context.Background()
	cred, _ := storage.GetCredential(ctx, "cred-u1")
	cred.Encrypted = key // legacy: the plaintext key itself
	storage.SaveCredential(ctx, cred)

	jwtAuth := service.NewJWTAuthenticator([]byte("unit-test-signing-secret"), "test")
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	rec := postJSON(t, srv, "/api/v1/auth/login", fmt.Sprintf(`{"api_key":%q}`, key))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (plaintext credential rejected), got %d", rec.Code)
	}
}

func TestHandleMe(t *testing.T) {
	storage := service.NewMemoryStorage()
	key := newTestAPIKeyUser(t, storage, "u1")

	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterAuthRoutes(nil)

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != "u1" {
		t.Fatalf("expected id u1, got %v", resp["id"])
	}
}

func TestHandleMeUnauthorized(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterAuthRoutes(nil)

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestV2RoutesRequireAuth(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterV2Routes()

	// Without auth, V2 endpoint should return 401.
	req := httptest.NewRequest("POST", "/v2/chat/stream", bytes.NewReader([]byte(`{"text":"hello"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestV2RoutesWithoutAuthenticator(t *testing.T) {
	// When no authenticator is configured, V2 routes should be open.
	srv := NewServer(&mockAgent{})
	srv.RegisterV2Routes()

	req := httptest.NewRequest("POST", "/v2/chat/stream", bytes.NewReader([]byte(`{"text":"hello"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Should not be 401; agent doesn't support V2 so it returns 501.
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("expected no auth required when authenticator is nil")
	}
}

// TestNewAppProductionFailsClosed locks the 22.1 assembly rule: in production
// mode with no identity source, business routes return 401 instead of
// staying anonymous.
func TestNewAppProductionFailsClosed(t *testing.T) {
	srv := NewApp(AppConfig{Production: true})
	srv.RegisterV2Routes()

	req := httptest.NewRequest("POST", "/v2/chat/stream", bytes.NewReader([]byte(`{"text":"hello"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 in production mode without identity source, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestNewAppJWTAuthJoinsChain locks the 22.1 assembly rule: passing only
// JWTAuth (no Authenticator) used to leave the server anonymous; NewApp now
// assembles it into the chain.
func TestNewAppJWTAuthJoinsChain(t *testing.T) {
	jwtAuth := service.NewJWTAuthenticator([]byte("unit-test-signing-secret"), "test")
	srv := NewApp(AppConfig{JWTAuth: jwtAuth})
	srv.RegisterV2Routes()

	// No Authorization header -> 401 proves the chain is active.
	req := httptest.NewRequest("POST", "/v2/chat/stream", bytes.NewReader([]byte(`{"text":"hello"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with JWTAuth-only assembly, got %d", rec.Code)
	}
}

// TestRegisterAuthRoutesIdempotent locks the 22.1 rule: calling
// RegisterAuthRoutes then RegisterAppRoutes (as examples/studio does) must
// not double-register and panic on the ServeMux.
func TestRegisterAuthRoutesIdempotent(t *testing.T) {
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte("unit-test-signing-secret"), "test")
	srv := NewApp(AppConfig{Storage: storage, JWTAuth: jwtAuth})
	srv.RegisterAuthRoutes(jwtAuth) // must not panic on the second call via:
	srv.RegisterAppRoutes(jwtAuth)
}
