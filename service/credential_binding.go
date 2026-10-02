// service/credential_binding.go realises the interactive-credential state
// machine (18.4). Bindings are two-phase: a credential is created PENDING
// (provider + label + binding reference only — no secret), and an external
// flow (OAuth callback, platform authorization, user action) moves it to a
// terminal state. Transitions are idempotent — the same target twice is a
// no-op success — and any replica can complete a pending binding, since the
// state lives in shared storage rather than one process.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrCredentialNotFound reports an unknown credential id.
	ErrCredentialNotFound = errors.New("credential not found")
	// ErrCredentialFinalized reports a transition into a different terminal
	// state than the one already recorded — terminal states are irreversible
	// (a repeated callback for the SAME state is a no-op success instead).
	ErrCredentialFinalized = errors.New("credential already finalized")
)

// terminalCredentialStatus reports whether s ends the binding lifecycle.
func terminalCredentialStatus(s CredentialStatus) bool {
	return s == CredentialAuthorized || s == CredentialFailed || s == CredentialCancelled
}

// CredentialConditionalWriter is the atomic transition capability (review
// finding: a plain read-judge-write lets two replicas that both read PENDING
// overwrite each other's terminal write — including replacing an already
// delivered secret with a later, different-outcome write). Storages that
// implement it serialize the judge+persist step:
//
//	SaveCredentialIfCurrent persists next only when the stored record's
//	normalized status is one of allowedCurrent; it returns the freshly
//
// stored record and swapped=true on success, or the CURRENTLY stored
// record and swapped=false when the condition no longer held (the caller
// re-judges). Errors are storage failures only.
//
// The gateway root's Memory/SQL/Redis storages all implement it; other
// Storage implementations fall back to the unconditional path below.
type CredentialConditionalWriter interface {
	SaveCredentialIfCurrent(ctx context.Context, id string, next *Credential, allowedCurrent ...CredentialStatus) (current *Credential, swapped bool, err error)
}

// TransitionCredential moves the credential idempotently to target:
//
//   - unknown id → ErrCredentialNotFound;
//   - current == target → no-op success (repeated callbacks, retried
//     deliveries);
//   - current is "" (pre-18.4) it reads as AUTHORIZED with the same
//     terminal-state rules;
//   - current is a different terminal state → ErrCredentialFinalized (the
//     write is refused, the recorded outcome stands);
//   - otherwise (PENDING, or the legacy-empty case moving nowhere) apply
//     runs first (it may attach the delivered secret / binding reference),
//     then the new status is persisted.
//
// apply is optional; it mutates the in-memory record before save (e.g. the
// authorize callback delivering the encrypted secret).
//
// Terminal states are irreversible BY CONSTRUCTION on storages implementing
// CredentialConditionalWriter: the persist step is conditional on the stored
// status still being PENDING ONLY. The target status is deliberately NOT in
// the allowed set: two authorize calls racing with DIFFERENT secrets would
// otherwise both satisfy "current ∈ {PENDING, AUTHORIZED}" and the later
// write would replace the already-delivered secret. With PENDING-only, the
// losing writer always sees swapped=false, re-reads the winner and either
// no-ops (same target, the winner's record — and secret — is returned) or
// reports ErrCredentialFinalized. An authorized secret cannot be
// overwritten by ANY later write — same or different outcome. On storages
// without the capability the unconditional save keeps the inherent
// read-judge-write window (same-outcome races remain harmless; documented
// limitation, not a guarantee).
//
// The transition also works on a copy: storages hand out shared pointers
// (MemoryStorage returns the stored *Credential itself), so mutating the
// fetched record would race concurrent readers and writers of the same
// object. Copying once keeps every goroutine on its own object;
// storage-level locking then serialises the final write.
func TransitionCredential(ctx context.Context, storage Storage, id string, target CredentialStatus, apply func(*Credential) error) (*Credential, error) {
	if !terminalCredentialStatus(target) && target != CredentialPending {
		return nil, fmt.Errorf("credential: unknown target status %q", target)
	}
	cred, err := storage.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrCredentialNotFound, id)
	}
	// Work on a copy from here on: the fetched pointer may be the stored
	// object itself (shared with concurrent transitions).
	copied := *cred
	cred = &copied
	current := cred.NormalizedStatus()
	if current == target {
		return cred, nil // idempotent no-op
	}
	if terminalCredentialStatus(current) {
		return cred, fmt.Errorf("%w: %s is %s, wanted %s", ErrCredentialFinalized, id, current, target)
	}
	if apply != nil {
		if err := apply(cred); err != nil {
			return nil, err
		}
	}
	cred.Status = target

	// Atomic path: the write is conditional on the stored status still
	// being PENDING — nothing else. A lost race (same OR different target)
	// re-judges against the persisted winner instead of silently
	// overwriting it; same-target idempotency is the re-judge outcome, not
	// a permission in the condition.
	if cw, ok := storage.(CredentialConditionalWriter); ok {
		cur, swapped, err := cw.SaveCredentialIfCurrent(ctx, id, cred, CredentialPending)
		if err != nil {
			return nil, fmt.Errorf("credential: persist transition: %w", err)
		}
		if swapped {
			return cred, nil
		}
		if cur == nil {
			return nil, fmt.Errorf("%w: %s", ErrCredentialNotFound, id)
		}
		if now := cur.NormalizedStatus(); now == target {
			return cur, nil // same outcome landed first — idempotent success
		} else {
			return cur, fmt.Errorf("%w: %s is %s, wanted %s", ErrCredentialFinalized, id, now, target)
		}
	}

	// Unconditional fallback (storages without the capability): the
	// read-judge-write window is inherent; last writer wins.
	cred.UpdatedAt = time.Now()
	if err := storage.SaveCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("credential: persist transition: %w", err)
	}
	return cred, nil
}
