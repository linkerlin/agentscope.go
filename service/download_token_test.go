package service

import (
	"strings"
	"testing"
	"time"
)

// TestDownloadTokenSignerRoundTripAndRefusals locks the 18.9 verdicts.
func TestDownloadTokenSignerRoundTripAndRefusals(t *testing.T) {
	s := NewDownloadTokenSigner([]byte("unit-secret"))
	now := time.Now()

	token, err := s.Mint("alice", "tts", "cosyvoice-v1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(token, "alice", "tts", "cosyvoice-v1", now); err != nil {
		t.Fatalf("round trip: %v", err)
	}

	// Cross-tenant.
	if err := s.Verify(token, "bob", "tts", "cosyvoice-v1", now); err != ErrDownloadTokenForbidden {
		t.Fatalf("cross-tenant: %v", err)
	}
	// Resource mismatch.
	if err := s.Verify(token, "alice", "tts", "other", now); err != ErrDownloadTokenMismatch {
		t.Fatalf("mismatch: %v", err)
	}
	// Kind mismatch.
	if err := s.Verify(token, "alice", "chat", "cosyvoice-v1", now); err != ErrDownloadTokenMismatch {
		t.Fatalf("kind mismatch: %v", err)
	}
	// Expired.
	if err := s.Verify(token, "alice", "tts", "cosyvoice-v1", now.Add(2*time.Minute)); err != ErrDownloadTokenExpired {
		t.Fatalf("expired: %v", err)
	}
	// Tampered body.
	dot := strings.LastIndex(token, ".")
	if err := s.Verify(token[:dot]+"A"+token[dot:], "alice", "tts", "cosyvoice-v1", now); err != ErrDownloadTokenMalformed {
		t.Fatalf("tampered: %v", err)
	}
	// Foreign secret.
	other := NewDownloadTokenSigner([]byte("other-secret"))
	foreign, _ := other.Mint("alice", "tts", "cosyvoice-v1", time.Minute)
	if err := s.Verify(foreign, "alice", "tts", "cosyvoice-v1", now); err != ErrDownloadTokenMalformed {
		t.Fatalf("foreign signature: %v", err)
	}
	// Garbage.
	if err := s.Verify("not-a-token", "alice", "tts", "cosyvoice-v1", now); err != ErrDownloadTokenMalformed {
		t.Fatalf("garbage: %v", err)
	}
}

// TestDownloadTokenSignerTTLAndFailsClosed: default and capped TTLs, and the
// empty-secret signer refuses to mint.
func TestDownloadTokenSignerTTLAndFailsClosed(t *testing.T) {
	s := NewDownloadTokenSigner(nil) // unconfigured
	if s.Enabled() {
		t.Fatal("empty secret must not be enabled")
	}
	if _, err := s.Mint("u", "chat", "m", time.Minute); err == nil {
		t.Fatal("empty secret must refuse to mint")
	}
	if err := s.Verify("x", "u", "chat", "m", time.Now()); err != ErrDownloadTokenMalformed {
		t.Fatalf("unconfigured verify: %v", err)
	}

	ok := NewDownloadTokenSigner([]byte("s"))
	// TTL capped at 1h.
	tok, err := ok.Mint("u", "chat", "m", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := ok.Verify(tok, "u", "chat", "m", time.Now().Add(2*time.Hour)); err != ErrDownloadTokenExpired {
		t.Fatalf("capped ttl must expire after 1h: %v", err)
	}
	if err := ok.Verify(tok, "u", "chat", "m", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatalf("within cap must pass: %v", err)
	}
	// Missing user/resource refuse.
	if _, err := ok.Mint("", "chat", "m", time.Minute); err == nil {
		t.Fatal("empty user must refuse")
	}
	if _, err := ok.Mint("u", "chat", "", time.Minute); err == nil {
		t.Fatal("empty resource must refuse")
	}
}
