package service

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestAnyAuthenticator_APIKeyOrJWT(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	user := &User{ID: "u1", Name: "Alice"}
	if err := storage.SaveUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveCredential(ctx, &Credential{
		ID: "c1", UserID: "u1", Provider: "api_key", Encrypted: "key-abc",
	}); err != nil {
		t.Fatal(err)
	}

	jwtAuth := NewJWTAuthenticator([]byte("secret"), "test")
	token, err := jwtAuth.GenerateToken("u1", 0)
	if err != nil {
		t.Fatal(err)
	}

	auth := NewAnyAuthenticator(
		NewAPIKeyAuthenticator(storage, ""),
		jwtAuth,
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "key-abc")
	outCtx, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("api key auth: %v", err)
	}
	if UserIDFromContext(outCtx) != "u1" {
		t.Fatalf("expected u1, got %q", UserIDFromContext(outCtx))
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	outCtx, err = auth.Authenticate(req2)
	if err != nil {
		t.Fatalf("jwt auth: %v", err)
	}
	if UserIDFromContext(outCtx) != "u1" {
		t.Fatalf("expected u1 from jwt, got %q", UserIDFromContext(outCtx))
	}
}
