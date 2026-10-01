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
// authorize callback delivering the encrypted secret). The read-judge-write
// window is inherent to the Storage interface; it is safe because every
// reachable end state is valid and writes are idempotent per target — a
// concurrent FAILED vs CANCELLED race ends in whichever terminal state
// landed last, never in a corrupt or half-written record.
func TransitionCredential(ctx context.Context, storage Storage, id string, target CredentialStatus, apply func(*Credential) error) (*Credential, error) {
	if !terminalCredentialStatus(target) && target != CredentialPending {
		return nil, fmt.Errorf("credential: unknown target status %q", target)
	}
	cred, err := storage.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrCredentialNotFound, id)
	}
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
	cred.UpdatedAt = time.Now()
	if err := storage.SaveCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("credential: persist transition: %w", err)
	}
	return cred, nil
}
