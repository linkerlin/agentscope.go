package service

import (
	"context"
	"sync"
	"testing"
)

// TestCredentialBindingStateMachine walks the full lifecycle on every
// Storage backend: create PENDING → authorize (idempotent repeat) → further
// transitions refused.
func TestCredentialBindingStateMachine(t *testing.T) {
	t.Run("memory", func(t *testing.T) { runBindingStateMachine(t, NewMemoryStorage()) })
	t.Run("sqlite", func(t *testing.T) { runBindingStateMachine(t, newTestSQLStorage(t)) })
	t.Run("redis", func(t *testing.T) {
		s, mr := setupRedisStorage(t)
		t.Cleanup(func() { mr.Close() })
		runBindingStateMachine(t, s)
	})
}

func runBindingStateMachine(t *testing.T, storage Storage) {
	t.Helper()
	ctx := context.Background()

	if err := storage.SaveCredential(ctx, &Credential{
		ID: "cb1", UserID: "u1", Provider: "dingtalk",
		Status: CredentialPending, BindingRef: "flow-1",
	}); err != nil {
		t.Fatal(err)
	}

	// PENDING credential is not usable.
	cred, err := storage.GetCredential(ctx, "cb1")
	if err != nil {
		t.Fatal(err)
	}
	if cred.NormalizedStatus() != CredentialPending {
		t.Fatalf("expected PENDING, got %s", cred.NormalizedStatus())
	}

	// Authorize delivers the secret.
	out, err := TransitionCredential(ctx, storage, "cb1", CredentialAuthorized, func(c *Credential) error {
		c.Encrypted = "enc-secret"
		return nil
	})
	if err != nil || out.NormalizedStatus() != CredentialAuthorized {
		t.Fatalf("authorize: %v %s", err, out.NormalizedStatus())
	}
	if out.Encrypted != "enc-secret" {
		t.Fatal("secret not attached")
	}

	// Repeated authorize (duplicate callback): idempotent no-op, secret untouched.
	out2, err := TransitionCredential(ctx, storage, "cb1", CredentialAuthorized, func(c *Credential) error {
		c.Encrypted = "MUTATED"
		return nil
	})
	if err != nil {
		t.Fatalf("repeated authorize must be a no-op success, got %v", err)
	}
	if out2.Encrypted == "MUTATED" {
		t.Fatal("apply ran on an idempotent repeat — the secret could be overwritten by a stale callback")
	}

	// Any further transition on a terminal state is refused.
	if _, err := TransitionCredential(ctx, storage, "cb1", CredentialCancelled, nil); err == nil {
		t.Fatal("cancel after authorize must be refused")
	}
	if _, err := TransitionCredential(ctx, storage, "cb1", CredentialFailed, nil); err == nil {
		t.Fatal("fail after authorize must be refused")
	}
}

// TestCredentialBindingCancelAndFail covers the other two terminals and the
// PENDING re-arm semantics (a failed flow stays FAILED — restart creates a
// new binding, it does not resurrect this one).
func TestCredentialBindingCancelAndFail(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	_ = storage.SaveCredential(ctx, &Credential{ID: "cb2", UserID: "u1", Provider: "p", Status: CredentialPending})

	if _, err := TransitionCredential(ctx, storage, "cb2", CredentialCancelled, nil); err != nil {
		t.Fatal(err)
	}
	// Repeated cancel: idempotent.
	if _, err := TransitionCredential(ctx, storage, "cb2", CredentialCancelled, nil); err != nil {
		t.Fatalf("repeated cancel must be idempotent: %v", err)
	}
	// Cancelled → authorize refused.
	if _, err := TransitionCredential(ctx, storage, "cb2", CredentialAuthorized, nil); err == nil {
		t.Fatal("authorize after cancel must be refused")
	}

	_ = storage.SaveCredential(ctx, &Credential{ID: "cb3", UserID: "u1", Provider: "p", Status: CredentialPending})
	if _, err := TransitionCredential(ctx, storage, "cb3", CredentialFailed, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionCredential(ctx, storage, "cb3", CredentialAuthorized, nil); err == nil {
		t.Fatal("authorize after fail must be refused")
	}
}

// TestCredentialBindingCrossReplicaResume locks the multi-replica recovery
// semantics: replica A creates the PENDING binding, replica B (a different
// Storage *instance* over the same backing store) completes it — the
// transition reads the shared record, applies, and persists.
func TestCredentialBindingCrossReplicaResume(t *testing.T) {
	path := t.TempDir() + "/binding.db"
	replicaA, err := NewSQLStorage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer replicaA.Close()
	ctx := context.Background()

	_ = replicaA.SaveCredential(ctx, &Credential{
		ID: "cb4", UserID: "u1", Provider: "dingtalk", Status: CredentialPending, BindingRef: "flow-9",
	})

	// Replica B opens the same database independently.
	replicaB, err := NewSQLStorage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer replicaB.Close()

	out, err := TransitionCredential(ctx, replicaB, "cb4", CredentialAuthorized, func(c *Credential) error {
		c.Encrypted = "enc-from-b"
		return nil
	})
	if err != nil || out.NormalizedStatus() != CredentialAuthorized {
		t.Fatalf("replica B resume: %v %s", err, out.NormalizedStatus())
	}
	// Replica A observes the same terminal state.
	cred, err := replicaA.GetCredential(ctx, "cb4")
	if err != nil || cred.NormalizedStatus() != CredentialAuthorized {
		t.Fatalf("replica A view: %v %s", err, cred.NormalizedStatus())
	}
}

// TestCredentialBindingConcurrentNoPendingLeft hammers concurrent
// transitions from PENDING: every goroutine lands on a legal outcome
// (no-op success, or refused because another terminal won), and afterwards
// the record is in exactly one terminal state — never stuck PENDING, never
// corrupt.
func TestCredentialBindingConcurrentNoPendingLeft(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	_ = storage.SaveCredential(ctx, &Credential{ID: "cb5", UserID: "u1", Provider: "p", Status: CredentialPending})

	targets := []CredentialStatus{CredentialAuthorized, CredentialFailed, CredentialCancelled, CredentialAuthorized}
	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(tg CredentialStatus) {
			defer wg.Done()
			_, _ = TransitionCredential(ctx, storage, "cb5", tg, nil) // outcome free per contract
		}(target)
	}
	wg.Wait()

	cred, err := storage.GetCredential(ctx, "cb5")
	if err != nil {
		t.Fatal(err)
	}
	if !terminalCredentialStatus(cred.NormalizedStatus()) {
		t.Fatalf("record stuck in non-terminal state: %s", cred.NormalizedStatus())
	}
}

// TestCredentialLegacyRowsReadAuthorized locks backward compatibility:
// pre-18.4 rows have no status field; they read as AUTHORIZED and reject
// binding transitions (they are not pending anything).
func TestCredentialLegacyRowsReadAuthorized(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	_ = storage.SaveCredential(ctx, &Credential{ID: "old1", UserID: "u1", Provider: "p"}) // no Status

	cred, err := storage.GetCredential(ctx, "old1")
	if err != nil {
		t.Fatal(err)
	}
	if cred.NormalizedStatus() != CredentialAuthorized {
		t.Fatalf("legacy row must read AUTHORIZED, got %q", cred.NormalizedStatus())
	}
	// Re-authorize is a no-op success (same target), not an error.
	if _, err := TransitionCredential(ctx, storage, "old1", CredentialAuthorized, nil); err != nil {
		t.Fatalf("re-authorize on legacy row must be a no-op: %v", err)
	}
	// But it is terminal: cancel refused.
	if _, err := TransitionCredential(ctx, storage, "old1", CredentialCancelled, nil); err == nil {
		t.Fatal("cancel on legacy (already-authorized) row must be refused")
	}
}
