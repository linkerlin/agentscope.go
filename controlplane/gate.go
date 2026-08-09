package controlplane

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// UserGate is a concrete, scoped blocker: a specific question the operator
// must answer before a gated lane may deliver. It is NOT a vague "waiting for
// owner" flag. A gate either blocks delivery (no fallback) or authorizes a
// single bounded fallback (Fallback != nil). Resolution sets Outcome; until
// then the gate is unresolved and ShouldRun reports ComputeOperatorGate.
type UserGate struct {
	GateID   string          `json:"gate_id"`
	GoalID   string          `json:"goal_id"`
	TodoID   string          `json:"todo_id,omitempty"`
	Question string          `json:"question"`
	Scope    DecisionScope   `json:"scope"`
	Fallback *FallbackPolicy `json:"fallback,omitempty"`
	Outcome  *GateOutcome    `json:"outcome,omitempty"` // nil = unresolved
	// Resolvers lists principals authorized to answer this gate (#5). Empty =
	// any authenticated principal may resolve (legacy/open). Non-empty = the
	// resolver's identity must appear here.
	Resolvers  []string  `json:"resolvers,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

// IsResolved reports whether the gate has been answered.
func (g UserGate) IsResolved() bool { return g.Outcome != nil }

// GateOutcome records who answered the gate and how.
type GateOutcome struct {
	Decision DecisionOutcome `json:"decision"` // approve|reject|cancel
	By       string          `json:"by,omitempty"`
	Note     string          `json:"note,omitempty"`
}

// FallbackPolicy declares an audited bounded action that may run DESPITE an
// open gate, because the gate's scope covers that action. The fallback never
// bypasses the gate: the gate stays open, the fallback is recorded, and it
// spends exactly one slot. Aligned with LoopX resolution matrix row 2.
type FallbackPolicy struct {
	Scope     DecisionScope `json:"scope"`      // must be covered by the gate
	Action    string        `json:"action"`     // bounded safe action description
	Audit     bool          `json:"audit"`      // must be recorded in the ledger
	SpendOnce bool          `json:"spend_once"` // execute -> spend exactly one slot
}

// ErrGateNotFound is returned by GateStore lookups for a missing gate.
var ErrGateNotFound = errors.New("controlplane: gate not found")

// ErrGateAlreadyResolved is returned when resolving an already-answered gate.
var ErrGateAlreadyResolved = errors.New("controlplane: gate already resolved")

// ErrUnauthorizedResolver is returned by ResolveGate when the gate declares a
// non-empty Resolvers list and the acting principal is not in it (#5). A gate's
// DecisionScope says WHAT the gate covers; Resolvers say WHO may answer it.
var ErrUnauthorizedResolver = errors.New("controlplane: principal not authorized to resolve gate")

// ErrFallbackScopeMismatch is returned when a fallback's scope is not covered
// by its gate. A fallback may never widen the gate's authority.
var ErrFallbackScopeMismatch = errors.New("controlplane: fallback scope not covered by gate")

// GateStore persists UserGate state scoped to a goal.
type GateStore interface {
	Upsert(ctx context.Context, g UserGate) error
	Get(ctx context.Context, goalID, gateID string) (UserGate, error)
	ListUnresolved(ctx context.Context, goalID string) ([]UserGate, error)
	// List returns ALL gates for the goal (resolved + unresolved), in stable
	// order. Used by capability lane-gate enforcement to confirm an APPROVED
	// gate exists (#3c) — ListUnresolved alone can't distinguish "approved"
	// from "never opened".
	List(ctx context.Context, goalID string) ([]UserGate, error)
}

// MemoryGateStore is a concurrency-safe in-process GateStore.
type MemoryGateStore struct {
	mu sync.RWMutex
	m  map[string]map[string]UserGate // goalID -> gateID -> gate
}

// NewMemoryGateStore returns an empty in-memory GateStore.
func NewMemoryGateStore() *MemoryGateStore {
	return &MemoryGateStore{m: make(map[string]map[string]UserGate)}
}

// Upsert inserts or replaces a gate. A new gate gets CreatedAt set. If the
// gate declares a Fallback whose scope is not covered by the gate's own scope,
// Upsert rejects it (fallback may never widen authority).
func (s *MemoryGateStore) Upsert(_ context.Context, g UserGate) error {
	if g.GateID == "" || g.GoalID == "" {
		return errors.New("controlplane: gate_id and goal_id required")
	}
	if g.Fallback != nil && !g.Scope.Covers(g.Fallback.Scope) {
		return ErrFallbackScopeMismatch
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	if s.m[g.GoalID] == nil {
		s.m[g.GoalID] = make(map[string]UserGate)
	}
	s.m[g.GoalID][g.GateID] = cloneGate(g)
	return nil
}

// Get returns the gate, or ErrGateNotFound.
func (s *MemoryGateStore) Get(_ context.Context, goalID, gateID string) (UserGate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gg, ok := s.m[goalID]
	if !ok {
		return UserGate{}, ErrGateNotFound
	}
	g, ok := gg[gateID]
	if !ok {
		return UserGate{}, ErrGateNotFound
	}
	return cloneGate(g), nil
}

// ListUnresolved returns gates for the goal that have no Outcome, in stable
// (created_at, gate_id) order so multi-gate ShouldRun surfaces a deterministic
// primary gate (#5 consistency with SQL backend's ORDER BY).
func (s *MemoryGateStore) ListUnresolved(_ context.Context, goalID string) ([]UserGate, error) {
	return s.listFiltered(goalID, false)
}

// List returns ALL gates for the goal (resolved + unresolved), stable-ordered.
func (s *MemoryGateStore) List(_ context.Context, goalID string) ([]UserGate, error) {
	return s.listFiltered(goalID, true)
}

func (s *MemoryGateStore) listFiltered(goalID string, includeResolved bool) ([]UserGate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gg, ok := s.m[goalID]
	if !ok {
		return nil, nil
	}
	out := make([]UserGate, 0, len(gg))
	for _, g := range gg {
		if !includeResolved && g.IsResolved() {
			continue
		}
		out = append(out, cloneGate(g))
	}
	// Stable order by CreatedAt then GateID (mirrors SQL ORDER BY).
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].GateID < out[j].GateID
	})
	return out, nil
}
