package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateAPIKeyUnpredictableAndWellFormed(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(key, "ask_") || len(key) < 40 {
			t.Fatalf("malformed key: %q", key)
		}
		if seen[key] {
			t.Fatal("duplicate key generated")
		}
		seen[key] = true
	}
}

func TestHashAPIKeyRoundTripAndConstantPrefix(t *testing.T) {
	key, _ := GenerateAPIKey()
	stored := HashAPIKey(key)
	if !IsHashedAPIKey(stored) {
		t.Fatalf("stored form not recognized as hashed: %q", stored)
	}
	if stored == key {
		t.Fatal("hash equals plaintext")
	}
	// Wrong keys must not verify.
	other, _ := GenerateAPIKey()
	if verifyAPIKeyHash(stored, other) {
		t.Fatal("different key verified")
	}
	if verifyAPIKeyHash(stored, key) != true {
		t.Fatal("correct key failed to verify")
	}
}

// TestVerifyAPIKeyHashRejectsPlaintextStorage locks the fail-closed rule:
// a credential stored as plaintext (legacy) never verifies.
func TestVerifyAPIKeyHashRejectsPlaintextStorage(t *testing.T) {
	key, _ := GenerateAPIKey()
	if verifyAPIKeyHash(key, key) {
		t.Fatal("plaintext-stored credential verified")
	}
	if IsHashedAPIKey(key) {
		t.Fatal("plaintext misclassified as hashed")
	}
}

func TestFindUserByAPIKey(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	storage.SaveUser(ctx, &User{ID: "u1", Name: "Alice"})
	key, _ := GenerateAPIKey()
	storage.SaveCredential(ctx, &Credential{
		ID: "c1", UserID: "u1", Provider: "api_key", Encrypted: HashAPIKey(key),
	})
	// Non-api_key credentials must be skipped by the scan.
	storage.SaveCredential(ctx, &Credential{
		ID: "c2", UserID: "u1", Provider: "openai", Encrypted: HashAPIKey(key),
	})

	u, err := FindUserByAPIKey(ctx, storage, key)
	if err != nil || u == nil || u.ID != "u1" {
		t.Fatalf("expected u1, got %v (%v)", u, err)
	}

	other, _ := GenerateAPIKey()
	if _, err := FindUserByAPIKey(ctx, storage, other); err == nil {
		t.Fatal("unknown key authenticated")
	}
	if _, err := FindUserByAPIKey(ctx, storage, ""); err == nil {
		t.Fatal("empty key authenticated")
	}
}

func TestRejectingAuthenticatorFailsClosed(t *testing.T) {
	auth := RejectingAuthenticator{}
	if _, err := auth.Authenticate(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Fatal("rejecting authenticator must fail")
	}
}

// TestCredentialJSONOmitsSecret locks the 22.1 redaction rule: the stored
// secret never appears in any JSON encoding of Credential.
func TestCredentialJSONOmitsSecret(t *testing.T) {
	key, _ := GenerateAPIKey()
	c := Credential{
		ID: "c1", UserID: "u1", Provider: "api_key", Label: "default",
		Encrypted: HashAPIKey(key),
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "encrypted") {
		t.Fatalf("JSON exposes encrypted field: %s", s)
	}
	if strings.Contains(s, c.Encrypted) {
		t.Fatalf("JSON leaks the stored secret: %s", s)
	}
}
