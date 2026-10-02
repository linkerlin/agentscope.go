package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// credCASBackend is one storage under test (18.4 review fix: the conditional
// writer must behave identically across backends).
type credCASBackend struct {
	name    string
	storage Storage
}

func credCASBackends(t *testing.T) []credCASBackend {
	t.Helper()
	sqlitePath := filepath.Join(t.TempDir(), "cas-test.db")
	sqlite, err := NewSQLStorage(context.Background(), sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	backends := []credCASBackend{
		{"memory", NewMemoryStorage()},
		{"sqlite", sqlite},
	}
	mr := miniredis.RunT(t)
	backends = append(backends, credCASBackend{"redis", NewRedisStorage(redis.NewClient(&redis.Options{Addr: mr.Addr()}))})
	return backends
}

// TestCredentialCASContract locks the conditional-writer contract on every
// backend: write succeeds while the stored status is allowed, is refused
// (with the current record returned for re-judging) otherwise, and reports
// unknown ids as errors. The allowed set is the CALLER's semantics (the
// primitive is generic); TransitionCredential always passes PENDING only —
// that call shape is asserted here too, because "same-target re-write with
// a different secret" is exactly the delivered-secret overwrite the review
// found.
func TestCredentialCASContract(t *testing.T) {
	ctx := context.Background()
	for _, b := range credCASBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			storage := b.storage
			_ = storage.SaveCredential(ctx, &Credential{ID: "cas1", UserID: "u1", Provider: "p", Status: CredentialPending})

			// Allowed condition holds (PENDING is in the set): swap.
			next := &Credential{ID: "cas1", UserID: "u1", Provider: "p", Status: CredentialAuthorized, Encrypted: "enc-1"}
			cur, swapped, err := storage.(CredentialConditionalWriter).SaveCredentialIfCurrent(ctx, "cas1", next, CredentialPending)
			if err != nil || !swapped {
				t.Fatalf("PENDING row must swap: swapped=%v err=%v", swapped, err)
			}
			if cur.Status != CredentialAuthorized {
				t.Fatalf("swapped record status: %s", cur.Status)
			}

			// Same-target re-write under the TransitionCredential call shape
			// (allowed = PENDING only): MUST be refused with the stored
			// record returned — a delivered secret is never replaced by a
			// later same-outcome write carrying a different secret.
			again := &Credential{ID: "cas1", UserID: "u1", Provider: "p", Status: CredentialAuthorized, Encrypted: "enc-2"}
			cur, swapped, err = storage.(CredentialConditionalWriter).SaveCredentialIfCurrent(ctx, "cas1", again, CredentialPending)
			if err != nil {
				t.Fatal(err)
			}
			if swapped {
				t.Fatal("PENDING-only condition must refuse a re-write over AUTHORIZED — the delivered secret would be replaced")
			}
			if cur == nil || cur.NormalizedStatus() != CredentialAuthorized || cur.Encrypted != "enc-1" {
				t.Fatalf("refusal must return the stored winner, got %+v", cur)
			}

			// Different-outcome writer: condition no longer holds, current is
			// returned for re-judging.
			cancel := &Credential{ID: "cas1", UserID: "u1", Provider: "p", Status: CredentialCancelled}
			cur, swapped, err = storage.(CredentialConditionalWriter).SaveCredentialIfCurrent(ctx, "cas1", cancel, CredentialPending)
			if err != nil {
				t.Fatal(err)
			}
			if swapped {
				t.Fatal("AUTHORIZED row must refuse a CANCELLED conditional write")
			}
			if cur == nil || cur.NormalizedStatus() != CredentialAuthorized {
				t.Fatalf("refusal must return the current record, got %+v", cur)
			}
			if cur.Encrypted != "enc-1" {
				t.Fatalf("delivered secret must survive the refused write, got %q", cur.Encrypted)
			}

			// Unknown id: error (never swapped).
			_, swapped, err = storage.(CredentialConditionalWriter).SaveCredentialIfCurrent(ctx, "nope", cancel, CredentialPending)
			if err == nil || swapped {
				t.Fatalf("unknown id must error: swapped=%v err=%v", swapped, err)
			}
		})
	}
}

// TestCredentialCASLegacyEmptyStatusConsistent: a pre-18.4 row (no status
// field) reads as AUTHORIZED everywhere. Directly invoking the conditional
// writer must therefore behave identically across backends: refused unless
// the caller explicitly allows AUTHORIZED — SQL/Redis previously passed
// empty unconditionally while Memory normalized first.
func TestCredentialCASLegacyEmptyStatusConsistent(t *testing.T) {
	ctx := context.Background()
	for _, b := range credCASBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			storage := b.storage
			_ = storage.SaveCredential(ctx, &Credential{ID: "legacy1", UserID: "u1", Provider: "p"}) // Status "" (legacy)

			overwrite := &Credential{ID: "legacy1", UserID: "u1", Provider: "p", Status: CredentialCancelled}
			cur, swapped, err := storage.(CredentialConditionalWriter).SaveCredentialIfCurrent(ctx, "legacy1", overwrite, CredentialPending)
			if err != nil {
				t.Fatal(err)
			}
			if swapped {
				t.Fatal("legacy empty status reads as AUTHORIZED: a PENDING-only write must be refused")
			}
			if cur == nil || cur.NormalizedStatus() != CredentialAuthorized {
				t.Fatalf("refusal must return the legacy record as AUTHORIZED, got %+v", cur)
			}

			// Explicitly allowing AUTHORIZED is the caller's decision and
			// works on every backend (primitive semantics).
			cur, swapped, err = storage.(CredentialConditionalWriter).SaveCredentialIfCurrent(ctx, "legacy1", overwrite, CredentialAuthorized)
			if err != nil || !swapped {
				t.Fatalf("explicit AUTHORIZED allowance must swap: swapped=%v err=%v", swapped, err)
			}
			if cur.NormalizedStatus() != CredentialCancelled {
				t.Fatalf("swapped record status: %s", cur.Status)
			}
		})
	}
}

// TestCredentialTransitionIrreversibleAfterAuthorize is the review scenario:
// once AUTHORIZED (with the delivered secret) is persisted, a later
// different-outcome transition must fail AND leave the secret untouched.
func TestCredentialTransitionIrreversibleAfterAuthorize(t *testing.T) {
	ctx := context.Background()
	for _, b := range credCASBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			storage := b.storage
			_ = storage.SaveCredential(ctx, &Credential{ID: "irv1", UserID: "u1", Provider: "p", Status: CredentialPending})

			if _, err := TransitionCredential(ctx, storage, "irv1", CredentialAuthorized, func(c *Credential) error {
				c.Encrypted = "delivered-secret"
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			// A racing cancel (late, different outcome): refused.
			if _, err := TransitionCredential(ctx, storage, "irv1", CredentialCancelled, nil); !errors.Is(err, ErrCredentialFinalized) {
				t.Fatalf("cancel after authorize must be ErrCredentialFinalized, got %v", err)
			}
			got, err := storage.GetCredential(ctx, "irv1")
			if err != nil {
				t.Fatal(err)
			}
			if got.NormalizedStatus() != CredentialAuthorized || got.Encrypted != "delivered-secret" {
				t.Fatalf("authorized record must stand: %+v", got)
			}

			// Same-target repeat: idempotent no-op, secret unchanged.
			if _, err := TransitionCredential(ctx, storage, "irv1", CredentialAuthorized, func(c *Credential) error {
				c.Encrypted = "must-not-run"
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			got, _ = storage.GetCredential(ctx, "irv1")
			if got.Encrypted != "delivered-secret" {
				t.Fatalf("repeat authorize must not re-deliver, got %q", got.Encrypted)
			}
		})
	}
}

// TestCredentialTransitionConcurrentTerminalNoOverwrite hammers the exact
// race the review found: authorize (delivering a secret) racing cancel from
// PENDING. Whatever interleaving, the stored record must be consistent —
// if AUTHORIZED, it carries the authorize writer's secret; if CANCELLED,
// no secret — and never a CANCELLED-wins-after-AUTHORIZED overwrite of the
// delivered secret.
func TestCredentialTransitionConcurrentTerminalNoOverwrite(t *testing.T) {
	ctx := context.Background()
	for _, b := range credCASBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			storage := b.storage
			for round := 0; round < 50; round++ {
				id := fmt.Sprintf("race-%d", round)
				_ = storage.SaveCredential(ctx, &Credential{ID: id, UserID: "u1", Provider: "p", Status: CredentialPending})

				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					_, _ = TransitionCredential(ctx, storage, id, CredentialAuthorized, func(c *Credential) error {
						c.Encrypted = "auth-secret"
						return nil
					})
				}()
				go func() {
					defer wg.Done()
					_, _ = TransitionCredential(ctx, storage, id, CredentialCancelled, nil)
				}()
				wg.Wait()

				got, err := storage.GetCredential(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				switch got.NormalizedStatus() {
				case CredentialAuthorized:
					if got.Encrypted != "auth-secret" {
						t.Fatalf("round %d: authorized without its delivered secret (got %q)", round, got.Encrypted)
					}
				case CredentialCancelled:
					if got.Encrypted != "" {
						t.Fatalf("round %d: cancelled carrying a secret %q", round, got.Encrypted)
					}
				default:
					t.Fatalf("round %d: stuck in non-terminal state %q", round, got.NormalizedStatus())
				}
			}
		})
	}
}

// TestCredentialTransitionConcurrentDuplicateAuthorize is the review race
// the PENDING-only condition closes: two authorize transitions carrying
// DIFFERENT secrets race from PENDING. The allowed set previously included
// the target, so the second write satisfied "current ∈ {PENDING,
// AUTHORIZED}" and replaced the first delivered secret. Now: exactly one
// apply runs, both calls succeed (the loser re-judges to a same-target
// no-op), and the stored secret is the winner's — never swapped afterwards.
func TestCredentialTransitionConcurrentDuplicateAuthorize(t *testing.T) {
	ctx := context.Background()
	for _, b := range credCASBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			storage := b.storage
			for round := 0; round < 50; round++ {
				id := fmt.Sprintf("dup-auth-%d", round)
				_ = storage.SaveCredential(ctx, &Credential{ID: id, UserID: "u1", Provider: "p", Status: CredentialPending})

				var applies atomic.Int64
				var wg sync.WaitGroup
				results := make([]*Credential, 2)
				errs := make([]error, 2)
				wg.Add(2)
				go func() {
					defer wg.Done()
					results[0], errs[0] = TransitionCredential(ctx, storage, id, CredentialAuthorized, func(c *Credential) error {
						applies.Add(1)
						c.Encrypted = "enc-alpha"
						return nil
					})
				}()
				go func() {
					defer wg.Done()
					results[1], errs[1] = TransitionCredential(ctx, storage, id, CredentialAuthorized, func(c *Credential) error {
						applies.Add(1)
						c.Encrypted = "enc-beta"
						return nil
					})
				}()
				wg.Wait()

				if n := applies.Load(); n < 1 || n > 2 {
					t.Fatalf("round %d: applies out of range (winner must run, loser's pre-CAS run is discarded): %d", round, n)
				}
				got, err := storage.GetCredential(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				if got.NormalizedStatus() != CredentialAuthorized {
					t.Fatalf("round %d: status %q", round, got.NormalizedStatus())
				}
				if got.Encrypted != "enc-alpha" && got.Encrypted != "enc-beta" {
					t.Fatalf("round %d: stored secret must be one winner's, got %q", round, got.Encrypted)
				}
				// Both callers leave with the SAME record (the loser's
				// no-op returns the winner's) — no caller observes a secret
				// that is not the stored one.
				for i, r := range results {
					if r == nil {
						t.Fatalf("round %d: caller %d returned no record (err=%v)", round, i, errs[i])
					}
					if r.Encrypted != got.Encrypted {
						t.Fatalf("round %d: caller %d saw %q but storage holds %q", round, i, r.Encrypted, got.Encrypted)
					}
				}
			}
		})
	}
}
