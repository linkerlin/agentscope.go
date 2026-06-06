package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/service"
)

func TestHandleRegister(t *testing.T) {
	storage := service.NewMemoryStorage()
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(nil)

	body := `{"name":"Alice"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

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
}

func TestHandleRegisterMissingName(t *testing.T) {
	storage := service.NewMemoryStorage()
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(nil)

	body := `{}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleLogin(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	user := &service.User{ID: "u1", Name: "Alice"}
	storage.SaveUser(ctx, user)

	jwtAuth := service.NewJWTAuthenticator([]byte("secret"), "test")
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	body := `{"user_id":"u1"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

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

func TestHandleLoginInvalidUser(t *testing.T) {
	storage := service.NewMemoryStorage()
	jwtAuth := service.NewJWTAuthenticator([]byte("secret"), "test")
	srv := NewServer(&mockAgent{}).WithStorage(storage)
	srv.RegisterAuthRoutes(jwtAuth)

	body := `{"user_id":"nonexistent"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleMe(t *testing.T) {
	storage := service.NewMemoryStorage()
	ctx := context.Background()
	user := &service.User{ID: "u1", Name: "Alice"}
	storage.SaveUser(ctx, user)
	storage.SaveCredential(ctx, &service.Credential{
		ID:        "c1",
		UserID:    "u1",
		Provider:  "api_key",
		Encrypted: "key-123",
	})

	apiAuth := service.NewAPIKeyAuthenticator(storage, "")
	srv := NewServer(&mockAgent{}).WithStorage(storage).WithAuthenticator(apiAuth)
	srv.RegisterAuthRoutes(nil)

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("X-API-Key", "key-123")
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
	body := `{"text":"hello"}`
	req := httptest.NewRequest("POST", "/v2/chat/stream", bytes.NewReader([]byte(body)))
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

	body := `{"text":"hello"}`
	req := httptest.NewRequest("POST", "/v2/chat/stream", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Should not be 401; agent doesn't support V2 so it returns 501.
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("expected no auth required when authenticator is nil")
	}
}
