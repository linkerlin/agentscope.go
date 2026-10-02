// service/download_token.go — model discovery download tokens (18.9): a
// signed, stateless grant binding ONE user to ONE resource for a bounded
// time. Verification refuses forged signatures, expired grants, grants for
// other users (cross-tenant), and grants for other resources.
package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Download token verdicts (18.9 acceptance: unauthorized, expired and
// cross-tenant access all refuse, each with a distinguishable reason).
var (
	// ErrDownloadTokenMalformed covers bad base64/JSON, a missing signature
	// and signature mismatch (forged or tampered).
	ErrDownloadTokenMalformed = errors.New("download token: malformed or bad signature")
	// ErrDownloadTokenExpired: the grant's deadline has passed.
	ErrDownloadTokenExpired = errors.New("download token: expired")
	// ErrDownloadTokenForbidden: the grant belongs to a different user.
	ErrDownloadTokenForbidden = errors.New("download token: issued to another user")
	// ErrDownloadTokenMismatch: the grant names a different resource.
	ErrDownloadTokenMismatch = errors.New("download token: resource mismatch")
)

// downloadTokenPayload is the signed body. Nonce defeats guessing (a token
// is 256-bit unguessable regardless, but the nonce also makes parallel
// grants for the same triple distinguishable in logs).
type downloadTokenPayload struct {
	UserID   string    `json:"uid"`
	Kind     string    `json:"kind"` // "chat" | "tts" | "embedding"
	Resource string    `json:"res"`
	Nonce    string    `json:"nonce"`
	Expires  time.Time `json:"exp"`
}

// DownloadTokenSigner mints and verifies download tokens under one secret.
type DownloadTokenSigner struct{ secret []byte }

// NewDownloadTokenSigner builds a signer. An empty secret mints nothing:
// Mint fails closed (callers surface the misconfiguration at assembly —
// principle 7).
func NewDownloadTokenSigner(secret []byte) *DownloadTokenSigner {
	return &DownloadTokenSigner{secret: secret}
}

// Enabled reports whether the signer can mint (non-empty secret).
func (s *DownloadTokenSigner) Enabled() bool { return s != nil && len(s.secret) > 0 }

// DefaultDownloadTokenTTL caps grants when the caller passes none.
const DefaultDownloadTokenTTL = 10 * time.Minute

// Mint issues a grant binding userID to (kind, resourceID) for ttl (capped
// at 1h — a download grant is short-lived by design).
func (s *DownloadTokenSigner) Mint(userID, kind, resourceID string, ttl time.Duration) (string, error) {
	if !s.Enabled() {
		return "", errors.New("download token: signer has no secret configured")
	}
	if userID == "" || resourceID == "" {
		return "", errors.New("download token: user and resource are required")
	}
	if ttl <= 0 {
		ttl = DefaultDownloadTokenTTL
	}
	if ttl > time.Hour {
		ttl = time.Hour
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("download token: nonce: %w", err)
	}
	payload := downloadTokenPayload{
		UserID:   userID,
		Kind:     kind,
		Resource: resourceID,
		Nonce:    base64.RawURLEncoding.EncodeToString(nonce),
		Expires:  time.Now().Add(ttl),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// Verify checks the token against the requesting user and the exact
// resource being accessed. now is injectable for tests.
func (s *DownloadTokenSigner) Verify(token, userID, kind, resourceID string, now time.Time) error {
	if !s.Enabled() {
		return ErrDownloadTokenMalformed
	}
	dot := -1
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot == len(token)-1 {
		return ErrDownloadTokenMalformed
	}
	body, err := base64.RawURLEncoding.DecodeString(token[:dot])
	if err != nil {
		return ErrDownloadTokenMalformed
	}
	sig, err := base64.RawURLEncoding.DecodeString(token[dot+1:])
	if err != nil {
		return ErrDownloadTokenMalformed
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ErrDownloadTokenMalformed
	}
	var payload downloadTokenPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ErrDownloadTokenMalformed
	}
	if now.After(payload.Expires) {
		return ErrDownloadTokenExpired
	}
	if payload.UserID != userID {
		return ErrDownloadTokenForbidden
	}
	if payload.Kind != kind || payload.Resource != resourceID {
		return ErrDownloadTokenMismatch
	}
	return nil
}
