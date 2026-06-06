package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAPIKeyAuthenticator(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	// Create a user with an API key credential.
	user := &User{ID: "u1", Name: "Alice"}
	if err := storage.SaveUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	cred := &Credential{
		ID:        "c1",
		UserID:    "u1",
		Provider:  "api_key",
		Label:     "test-key",
		Encrypted: "secret-key-123",
	}
	if err := storage.SaveCredential(ctx, cred); err != nil {
		t.Fatal(err)
	}

	auth := NewAPIKeyAuthenticator(storage, "")

	// Valid key.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "secret-key-123")
	actx, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if UserIDFromContext(actx) != "u1" {
		t.Fatalf("expected user u1, got %s", UserIDFromContext(actx))
	}
	if UserFromContext(actx) == nil || UserFromContext(actx).Name != "Alice" {
		t.Fatal("expected user object in context")
	}

	// Missing header.
	req2 := httptest.NewRequest("GET", "/", nil)
	if _, err := auth.Authenticate(req2); err == nil {
		t.Fatal("expected error for missing header")
	}

	// Invalid key.
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("X-API-Key", "wrong-key")
	if _, err := auth.Authenticate(req3); err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestJWTAuthenticator(t *testing.T) {
	secret := []byte("my-secret")
	auth := NewJWTAuthenticator(secret, "test-issuer")

	// Generate a valid token.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u2",
		"iss": "test-issuer",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	actx, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if UserIDFromContext(actx) != "u2" {
		t.Fatalf("expected user u2, got %s", UserIDFromContext(actx))
	}

	// Missing header.
	req2 := httptest.NewRequest("GET", "/", nil)
	if _, err := auth.Authenticate(req2); err == nil {
		t.Fatal("expected error for missing header")
	}

	// Invalid token.
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("Authorization", "Bearer invalid.token.here")
	if _, err := auth.Authenticate(req3); err == nil {
		t.Fatal("expected error for invalid token")
	}

	// Wrong signing secret.
	tokenWrong := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u3",
		"iss": "test-issuer",
	})
	tokenWrongStr, _ := tokenWrong.SignedString([]byte("other-secret"))
	req4 := httptest.NewRequest("GET", "/", nil)
	req4.Header.Set("Authorization", "Bearer "+tokenWrongStr)
	if _, err := auth.Authenticate(req4); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestAuthMiddleware(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	user := &User{ID: "u1", Name: "Alice"}
	storage.SaveUser(ctx, user)
	storage.SaveCredential(ctx, &Credential{
		ID:        "c1",
		UserID:    "u1",
		Provider:  "api_key",
		Encrypted: "key-123",
	})

	auth := NewAPIKeyAuthenticator(storage, "")
	handler := AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, UserIDFromContext(r.Context()))
	}))

	// Valid request.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "key-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "u1" {
		t.Fatalf("expected u1, got %s", rec.Body.String())
	}

	// Unauthorized.
	req2 := httptest.NewRequest("GET", "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec2.Code)
	}
}
