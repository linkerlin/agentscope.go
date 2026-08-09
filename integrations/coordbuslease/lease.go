// Package coordbuslease implements controlplane.LeaseStore over a
// messagebus.CoordBus. It composes CoordBus.Lock (mutual exclusion with TTL
// auto-release) and CoordBus.Registry (owner bookkeeping) to provide
// owner-guarded hard leases — the cross-process counterpart of controlplane's
// in-process MemoryLeaseStore.
//
// This adapter is the "reuse CoordBus.Lock" wiring promised in the LoopX
// evolution plan: controlplane stays dependency-free and defines the
// LeaseStore contract; this integration package binds it to the existing
// coordination bus (LocalBus in-process, RedisBus across processes).
//
// Cross-process semantics: Acquire and Inspect are fully correct across
// processes (they go through the shared bus + registry). Renew, Release, and
// Transfer are correct when invoked from the process that holds the lock
// (the common case: the agent that acquired a lease renews/releases it). The
// CoordBus Cancel handle is process-local by design, so releasing a lock from
// a different process relies on TTL — the same discipline as Redis redlock.
//
// ponytail: renewal has a tiny release+re-lock race window, and cross-process
// Transfer relies on TTL. If strict renew/transfer correctness is required,
// upgrade to a dedicated Redis LeaseStore using Lua CAS (SET NX PX with an
// owner-token value + token-checked release), as messagebus already does for
// its own Lock.
package coordbuslease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/messagebus"
)

// tryLockBudget is the short context budget used to turn CoordBus.Lock (which
// blocks until acquired or ctx-done) into a non-blocking try-lock.
const tryLockBudget = 50 * time.Millisecond

// leaseRec is the owner record stored in the CoordBus registry.
type leaseRec struct {
	Owner     string    `json:"owner"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Adapter implements controlplane.LeaseStore over a messagebus.CoordBus.
type Adapter struct {
	cb    messagebus.CoordBus
	ns    string // registry namespace
	mu    sync.Mutex
	locks map[string]messagebus.Cancel // key -> held Cancel (process-local)
}

// New returns a LeaseStore backed by cb. The registry namespace defaults to
// "cp_lease"; pass NewWithNamespace for isolation.
func New(cb messagebus.CoordBus) *Adapter {
	return &Adapter{cb: cb, ns: "cp_lease", locks: make(map[string]messagebus.Cancel)}
}

// NewWithNamespace returns an Adapter using a custom registry namespace, so
// independent Kernel instances sharing one bus do not collide.
func NewWithNamespace(cb messagebus.CoordBus, ns string) *Adapter {
	a := New(cb)
	if ns != "" {
		a.ns = ns
	}
	return a
}

func (a *Adapter) key(goalID, todoID string) string { return goalID + "\x00" + todoID }

// Acquire takes a lease if the todo is free or already held by owner; otherwise
// returns controlplane.ErrLeaseHeld (naming the current owner).
func (a *Adapter) Acquire(ctx context.Context, goalID, todoID, owner string, ttl time.Duration) (*controlplane.Lease, error) {
	if owner == "" {
		return nil, errors.New("coordbuslease: lease owner required")
	}
	if ttl <= 0 {
		ttl = controlplane.DefaultLeaseTTL
	}
	if ttl > controlplane.MaxLeaseTTL {
		ttl = controlplane.MaxLeaseTTL
	}
	key := a.key(goalID, todoID)

	// Fast path: this process already holds the lock for owner -> renew.
	a.mu.Lock()
	if cancel, ok := a.locks[key]; ok && cancel != nil {
		rec, _ := a.readRec(ctx, key)
		if rec != nil && rec.Owner == owner {
			expires := time.Now().UTC().Add(ttl)
			_ = a.writeRec(ctx, key, leaseRec{Owner: owner, ExpiresAt: expires})
			a.mu.Unlock()
			return &controlplane.Lease{GoalID: goalID, TodoID: todoID, Owner: owner,
				AcquiredAt: time.Now().UTC(), ExpiresAt: expires}, nil
		}
	}
	a.mu.Unlock()

	// Try to acquire the CoordBus lock (non-blocking via a short ctx budget).
	tctx, cancel := context.WithTimeout(ctx, tryLockBudget)
	defer cancel()
	lockCancel, err := a.cb.Lock(tctx, key, ttl)
	if err != nil {
		// Lock not acquired: report who holds it, if anyone.
		if rec, _ := a.readRec(ctx, key); rec != nil && time.Now().UTC().Before(rec.ExpiresAt) {
			return nil, fmt.Errorf("%w: held by %s", controlplane.ErrLeaseHeld, rec.Owner)
		}
		return nil, fmt.Errorf("%w: held by another owner", controlplane.ErrLeaseHeld)
	}

	expires := time.Now().UTC().Add(ttl)
	a.mu.Lock()
	a.locks[key] = lockCancel
	a.mu.Unlock()
	_ = a.writeRec(ctx, key, leaseRec{Owner: owner, ExpiresAt: expires})
	return &controlplane.Lease{GoalID: goalID, TodoID: todoID, Owner: owner,
		AcquiredAt: time.Now().UTC(), ExpiresAt: expires}, nil
}

// Renew extends the lease ttl. Requires the caller to be the current owner and
// to be the process holding the lock.
func (a *Adapter) Renew(ctx context.Context, goalID, todoID, owner string, ttl time.Duration) (*controlplane.Lease, error) {
	if ttl <= 0 {
		ttl = controlplane.DefaultLeaseTTL
	}
	if ttl > controlplane.MaxLeaseTTL {
		ttl = controlplane.MaxLeaseTTL
	}
	key := a.key(goalID, todoID)
	rec, err := a.readRec(ctx, key)
	if err != nil || rec == nil {
		return nil, controlplane.ErrLeaseNotFound
	}
	if rec.Owner != owner {
		return nil, controlplane.ErrLeaseOwnerMismatch
	}
	// Release the held lock and re-acquire with the new ttl. The release+relock
	// window is the documented ponytail ceiling.
	a.mu.Lock()
	if cancel, ok := a.locks[key]; ok && cancel != nil {
		cancel()
		delete(a.locks, key)
	}
	a.mu.Unlock()
	lockCancel, err := a.cb.Lock(ctx, key, ttl)
	if err != nil {
		return nil, fmt.Errorf("%w: re-acquire failed during renew", controlplane.ErrLeaseHeld)
	}
	expires := time.Now().UTC().Add(ttl)
	a.mu.Lock()
	a.locks[key] = lockCancel
	a.mu.Unlock()
	_ = a.writeRec(ctx, key, leaseRec{Owner: owner, ExpiresAt: expires})
	return &controlplane.Lease{GoalID: goalID, TodoID: todoID, Owner: owner,
		AcquiredAt: time.Now().UTC(), ExpiresAt: expires}, nil
}

// Transfer hands the lease to newOwner. Only the current owner may transfer.
// The CoordBus lock itself is not re-acquired; cross-process release by the
// new owner relies on TTL (documented ceiling).
func (a *Adapter) Transfer(ctx context.Context, goalID, todoID, owner, newOwner string) (*controlplane.Lease, error) {
	if newOwner == "" {
		return nil, errors.New("coordbuslease: new owner required")
	}
	key := a.key(goalID, todoID)
	rec, err := a.readRec(ctx, key)
	if err != nil || rec == nil {
		return nil, controlplane.ErrLeaseNotFound
	}
	if rec.Owner != owner {
		return nil, controlplane.ErrLeaseOwnerMismatch
	}
	rec.Owner = newOwner
	if err := a.writeRec(ctx, key, *rec); err != nil {
		return nil, err
	}
	return &controlplane.Lease{GoalID: goalID, TodoID: todoID, Owner: newOwner,
		AcquiredAt: time.Now().UTC(), ExpiresAt: rec.ExpiresAt}, nil
}

// Release drops the caller's lease. Requires the caller to be the current
// owner. If this process holds the CoordBus lock it is released immediately;
// otherwise the lock expires by TTL.
func (a *Adapter) Release(ctx context.Context, goalID, todoID, owner string) error {
	key := a.key(goalID, todoID)
	rec, err := a.readRec(ctx, key)
	if err != nil || rec == nil {
		return nil // nothing to release
	}
	if rec.Owner != owner {
		return controlplane.ErrLeaseOwnerMismatch
	}
	a.mu.Lock()
	if cancel, ok := a.locks[key]; ok && cancel != nil {
		cancel()
		delete(a.locks, key)
	}
	a.mu.Unlock()
	_ = a.writeRec(ctx, key, leaseRec{}) // empty value deletes the registry entry
	return nil
}

// Inspect returns the current lease or controlplane.ErrLeaseNotFound, treating
// an expired record as absent (lazy expiry on the registry side).
func (a *Adapter) Inspect(ctx context.Context, goalID, todoID string) (*controlplane.Lease, error) {
	key := a.key(goalID, todoID)
	rec, err := a.readRec(ctx, key)
	if err != nil || rec == nil {
		return nil, controlplane.ErrLeaseNotFound
	}
	if !time.Now().UTC().Before(rec.ExpiresAt) {
		_ = a.writeRec(ctx, key, leaseRec{}) // reap expired
		return nil, controlplane.ErrLeaseNotFound
	}
	return &controlplane.Lease{GoalID: goalID, TodoID: todoID, Owner: rec.Owner,
		ExpiresAt: rec.ExpiresAt}, nil
}

func (a *Adapter) readRec(ctx context.Context, key string) (*leaseRec, error) {
	b, err := a.cb.RegistryGet(ctx, a.ns, key)
	if err != nil || len(b) == 0 {
		return nil, nil
	}
	var rec leaseRec
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, nil
	}
	return &rec, nil
}

func (a *Adapter) writeRec(ctx context.Context, key string, rec leaseRec) error {
	if rec.Owner == "" && rec.ExpiresAt.IsZero() {
		return a.cb.RegistrySet(ctx, a.ns, key, nil) // delete
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return a.cb.RegistrySet(ctx, a.ns, key, b)
}
