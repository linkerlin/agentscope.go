package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DefaultLeaseTTL is the default hard-lease duration (LoopX: 45 min). The
// contention unit is a single (GoalID, TodoID) pair, NOT the whole goal, so
// different todos under one goal may proceed in parallel.
const DefaultLeaseTTL = 45 * time.Minute

// MaxLeaseTTL caps a lease duration (LoopX: 24 h).
const MaxLeaseTTL = 24 * time.Hour

// Lease is a hard, TTL-bounded ownership claim on one todo, held by one owner.
// Unlike the soft ClaimedBy field, a lease is the contention primitive for
// hosts with concurrent-write problems. The store is the durability boundary;
// P2 ships an in-process implementation and a LeaseStore interface that a
// messagebus.CoordBus adapter can satisfy for cross-process use.
type Lease struct {
	GoalID     string    `json:"goal_id"`
	TodoID     string    `json:"todo_id"`
	Owner      string    `json:"owner"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// IsValid reports whether the lease is present and unexpired at now.
func (l *Lease) IsValid(now time.Time) bool {
	return l != nil && now.Before(l.ExpiresAt)
}

// ErrLeaseHeld is returned by Acquire when another owner holds a valid lease.
var ErrLeaseHeld = errors.New("controlplane: lease held by another owner")

// ErrLeaseNotFound is returned when no lease exists for the todo.
var ErrLeaseNotFound = errors.New("controlplane: no lease for todo")

// ErrLeaseOwnerMismatch is returned by Renew/Transfer/Release when the caller
// is not the current owner.
var ErrLeaseOwnerMismatch = errors.New("controlplane: caller is not the lease owner")

// LeaseStore is the hard-lease primitive. Operations are per (GoalID, TodoID):
// acquire (CAS), renew (extend TTL), transfer (hand off to another owner),
// release, inspect. This is the contract a messagebus.CoordBus.Lock adapter
// satisfies; controlplane does not import messagebus directly.
type LeaseStore interface {
	Acquire(ctx context.Context, goalID, todoID, owner string, ttl time.Duration) (*Lease, error)
	Renew(ctx context.Context, goalID, todoID, owner string, ttl time.Duration) (*Lease, error)
	Transfer(ctx context.Context, goalID, todoID, owner, newOwner string) (*Lease, error)
	Release(ctx context.Context, goalID, todoID, owner string) error
	Inspect(ctx context.Context, goalID, todoID string) (*Lease, error)
}

// MemoryLeaseStore is a concurrency-safe in-process LeaseStore with lazy TTL
// expiry (no background goroutine; expired leases are reaped on next access).
//
// ponytail: lazy expiry only. A background reaper adds complexity for no
// correctness gain; expired entries cost O(1) on access. Upgrade to a
// Redis-backed adapter (messagebus.CoordBus.Lock) when cross-process leases
// are needed.
type MemoryLeaseStore struct {
	mu sync.Mutex
	m  map[string]*Lease
}

// NewMemoryLeaseStore returns an empty in-process LeaseStore.
func NewMemoryLeaseStore() *MemoryLeaseStore {
	return &MemoryLeaseStore{m: make(map[string]*Lease)}
}

func leaseKey(goalID, todoID string) string { return goalID + "\x00" + todoID }

func (s *MemoryLeaseStore) reapLocked(key string, now time.Time) {
	if l, ok := s.m[key]; ok && !l.IsValid(now) {
		delete(s.m, key)
	}
}

// Acquire takes a lease if the todo is free or already held by owner; otherwise
// returns ErrLeaseHeld. ttl<=0 uses DefaultLeaseTTL; ttl>MaxLeaseTTL is capped.
func (s *MemoryLeaseStore) Acquire(_ context.Context, goalID, todoID, owner string, ttl time.Duration) (*Lease, error) {
	if owner == "" {
		return nil, errors.New("controlplane: lease owner required")
	}
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	if ttl > MaxLeaseTTL {
		ttl = MaxLeaseTTL
	}
	now := time.Now().UTC()
	key := leaseKey(goalID, todoID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked(key, now)
	if cur, ok := s.m[key]; ok && cur.Owner != owner {
		return nil, fmt.Errorf("%w: held by %s", ErrLeaseHeld, cur.Owner)
	}
	l := &Lease{
		GoalID: goalID, TodoID: todoID, Owner: owner,
		AcquiredAt: now, ExpiresAt: now.Add(ttl),
	}
	s.m[key] = l
	return l, nil
}

// Renew extends the lease ttl. Only the current owner may renew.
func (s *MemoryLeaseStore) Renew(_ context.Context, goalID, todoID, owner string, ttl time.Duration) (*Lease, error) {
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	if ttl > MaxLeaseTTL {
		ttl = MaxLeaseTTL
	}
	now := time.Now().UTC()
	key := leaseKey(goalID, todoID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked(key, now)
	cur, ok := s.m[key]
	if !ok {
		return nil, ErrLeaseNotFound
	}
	if cur.Owner != owner {
		return nil, ErrLeaseOwnerMismatch
	}
	cur.ExpiresAt = now.Add(ttl)
	return cur, nil
}

// Transfer hands the lease to newOwner. Only the current owner may transfer.
func (s *MemoryLeaseStore) Transfer(_ context.Context, goalID, todoID, owner, newOwner string) (*Lease, error) {
	if newOwner == "" {
		return nil, errors.New("controlplane: new owner required")
	}
	now := time.Now().UTC()
	key := leaseKey(goalID, todoID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked(key, now)
	cur, ok := s.m[key]
	if !ok {
		return nil, ErrLeaseNotFound
	}
	if cur.Owner != owner {
		return nil, ErrLeaseOwnerMismatch
	}
	cur.Owner = newOwner
	cur.AcquiredAt = now
	return cur, nil
}

// Release drops the lease. Only the current owner may release; releasing a
// missing or expired lease is a no-op.
func (s *MemoryLeaseStore) Release(_ context.Context, goalID, todoID, owner string) error {
	now := time.Now().UTC()
	key := leaseKey(goalID, todoID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked(key, now)
	cur, ok := s.m[key]
	if !ok {
		return nil
	}
	if cur.Owner != owner {
		return ErrLeaseOwnerMismatch
	}
	delete(s.m, key)
	return nil
}

// Inspect returns the current lease (or ErrLeaseNotFound), reaping an expired
// entry first.
func (s *MemoryLeaseStore) Inspect(_ context.Context, goalID, todoID string) (*Lease, error) {
	now := time.Now().UTC()
	key := leaseKey(goalID, todoID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked(key, now)
	cur, ok := s.m[key]
	if !ok {
		return nil, ErrLeaseNotFound
	}
	cp := *cur
	return &cp, nil
}
