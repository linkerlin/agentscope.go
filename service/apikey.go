package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// API keys are high-entropy random strings, so a plain SHA-256 digest (no
// salt) is the storage form — the same trade-off GitHub/Google make for
// token identifiers. The "sha256:" prefix distinguishes hashed credentials
// from legacy plaintext ones; the verifier rejects the latter outright, so
// an un-migrated database authenticates nobody instead of everybody.
const apiKeyHashPrefix = "sha256:"

// GenerateAPIKey returns a new random API key (32 bytes of crypto/rand,
// URL-safe base64). Unlike the old timestamp-based IDs these are unguessable.
func GenerateAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("apikey: entropy source failed: %w", err)
	}
	return "ask_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashAPIKey returns the storage form of an API key ("sha256:<hex>").
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return apiKeyHashPrefix + hex.EncodeToString(sum[:])
}

// IsHashedAPIKey reports whether a stored credential value is in hashed form.
func IsHashedAPIKey(stored string) bool {
	return strings.HasPrefix(stored, apiKeyHashPrefix)
}

// verifyAPIKeyHash reports whether key matches a hashed storage form, in
// constant time. Legacy plaintext credentials never match.
func verifyAPIKeyHash(stored, key string) bool {
	if !IsHashedAPIKey(stored) {
		return false
	}
	sum := sha256.Sum256([]byte(key))
	got := hex.EncodeToString(sum[:])
	want := strings.TrimPrefix(stored, apiKeyHashPrefix)
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// FindUserByAPIKey verifies key against every "api_key" credential and
// returns the owning user. It is the single verification path shared by the
// API-key authenticator and the login endpoint, so both enforce hashing.
// The scan is linear over users; suitable for the bundled dev/test storages
// (a production deployment fronting many users should index by hash).
func FindUserByAPIKey(ctx context.Context, storage Storage, key string) (*User, error) {
	if key == "" {
		return nil, fmt.Errorf("apikey: empty key")
	}
	users, err := storage.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("apikey: list users failed: %w", err)
	}
	for _, u := range users {
		creds, err := storage.ListCredentialsByUser(ctx, u.ID)
		if err != nil {
			continue
		}
		for _, c := range creds {
			if c.Provider != "api_key" {
				continue
			}
			if verifyAPIKeyHash(c.Encrypted, key) {
				return u, nil
			}
		}
	}
	return nil, fmt.Errorf("apikey: invalid API key")
}

// RejectingAuthenticator denies every request. NewApp installs it in
// production mode when no identity source is configured, so business routes
// fail closed (401) instead of silently staying anonymous. The server still
// boots — health endpoints keep working — and the constant error string
// makes the misconfiguration visible in logs.
type RejectingAuthenticator struct{}

// Authenticate implements Authenticator; it always fails.
func (RejectingAuthenticator) Authenticate(r *http.Request) (context.Context, error) {
	return r.Context(), fmt.Errorf("no authenticator configured (production mode fails closed)")
}
