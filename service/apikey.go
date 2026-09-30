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

// APIKeyCredentialFinder is an optional Storage extension, implemented by
// SQLStorage, RedisStorage and MemoryStorage: backends that can locate
// credentials by their stored API-key hash. FindUserByAPIKey uses it to turn
// the O(users × credentials) verification scan into one indexed lookup.
type APIKeyCredentialFinder interface {
	FindCredentialsByHash(ctx context.Context, keyHash string) ([]*Credential, error)
}

// apiKeyIndexKey is the hash-index key for a credential, or "" when the
// credential carries no hashed API key (other providers, legacy plaintext).
// All backends index by the stored hash form ("sha256:<hex>") — the same
// value verifyAPIKeyHash expects, so index writes need no transformation.
func apiKeyIndexKey(c *Credential) string {
	if c.Provider == "api_key" && IsHashedAPIKey(c.Encrypted) {
		return c.Encrypted
	}
	return ""
}

// FindUserByAPIKey verifies key against api_key credentials and returns the
// owning user. It is the single verification path shared by the API-key
// authenticator and the login endpoint, so both enforce hashing.
//
// Storages implementing APIKeyCredentialFinder resolve the credential with
// one hash-indexed lookup (invalid keys are rejected in constant time);
// every candidate is still verified in constant time against the presented
// key before its owner is returned, so index pollution or a theoretical
// digest collision cannot authenticate the wrong key. Other storages fall
// back to a linear scan over users and credentials.
func FindUserByAPIKey(ctx context.Context, storage Storage, key string) (*User, error) {
	if key == "" {
		return nil, fmt.Errorf("apikey: empty key")
	}
	if finder, ok := storage.(APIKeyCredentialFinder); ok {
		return findUserByAPIKeyIndexed(ctx, storage, finder, key)
	}
	return findUserByAPIKeyScan(ctx, storage, key)
}

func findUserByAPIKeyIndexed(ctx context.Context, storage Storage, finder APIKeyCredentialFinder, key string) (*User, error) {
	creds, err := finder.FindCredentialsByHash(ctx, HashAPIKey(key))
	if err != nil {
		return nil, fmt.Errorf("apikey: hash lookup failed: %w", err)
	}
	for _, c := range creds {
		if c.Provider != "api_key" {
			continue
		}
		if !verifyAPIKeyHash(c.Encrypted, key) {
			continue
		}
		u, err := storage.GetUser(ctx, c.UserID)
		if err != nil {
			// Index entry points at a deleted user; keep looking.
			continue
		}
		return u, nil
	}
	return nil, fmt.Errorf("apikey: invalid API key")
}

// findUserByAPIKeyScan is the pre-index linear fallback for storages without
// a hash index (fine for bundled dev/test storages; a production deployment
// should use an indexed backend).
func findUserByAPIKeyScan(ctx context.Context, storage Storage, key string) (*User, error) {
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
