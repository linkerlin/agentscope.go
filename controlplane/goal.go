package controlplane

import (
	"context"
	"errors"
	"sync"
	"time"
)

// AuthoritySrc replaces implicit model memory with reviewable context: who or
// what is allowed to change this goal. Aligned with LoopX "authority sources".
type AuthoritySrc struct {
	Kind  string `json:"kind"`  // e.g. "owner", "repo", "policy"
	Value string `json:"value"` // e.g. "github.com/owner/repo", "alice@example.com"
}

// Goal is a durable lifetime objective: the source of truth that survives
// across many turns, days, and executors. It is narrow enough that automation
// makes one bounded, verifiable move, yet stable enough that a future agent
// can recover the objective, its scope, its authority, and the next safe
// transition. This is the LoopX "lifetime goal invariant".
type Goal struct {
	ID            string         `json:"id"`
	OwnerUserID   string         `json:"owner_user_id,omitempty"` // tenant scope (#5); empty = legacy/global
	CapabilityID  string         `json:"capability_id,omitempty"` // lane this goal runs, e.g. "issue-fix" (#3c)
	Objective     string         `json:"objective"`
	Scope         []string       `json:"scope,omitempty"`     // explicit boundaries / non-goals
	Authority     []AuthoritySrc `json:"authority,omitempty"` // who may change this goal
	State         GoalState      `json:"state"`               // active|paused|completed|abandoned
	CurrentTodoID string         `json:"current_todo_id,omitempty"`
	Quota         Quota          `json:"quota"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// goalTransitions is the legal state-transition table. Terminal states have
// no outgoing edges. Making illegal states hard to express (AGENTS.md rule).
var goalTransitions = map[GoalState]map[GoalState]bool{
	GoalActive: {
		GoalPaused:    true,
		GoalCompleted: true,
		GoalAbandoned: true,
	},
	GoalPaused: {
		GoalActive:    true,
		GoalAbandoned: true,
	},
	GoalCompleted: {},
	GoalAbandoned: {},
}

// LegalGoalTransition reports whether moving from -> to is permitted.
func LegalGoalTransition(from, to GoalState) bool {
	if to == from {
		return true
	}
	return goalTransitions[from][to]
}

// ErrGoalNotFound is returned by GoalStore lookups for a missing goal.
var ErrGoalNotFound = errors.New("controlplane: goal not found")

// GoalStore persists Goal state. Implementations: in-memory (P0), SQL and
// Redis later (wired through service.Storage). The store is the durability
// boundary; the Kernel holds no durable state of its own.
type GoalStore interface {
	Get(ctx context.Context, id string) (*Goal, error)
	Upsert(ctx context.Context, g *Goal) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*Goal, error)
}

// MemoryGoalStore is a concurrency-safe in-process GoalStore for P0 and tests.
// ponytail: in-memory only; upgrade to SQL/Redis via service.Storage when
// cross-process durability is needed (P1+).
type MemoryGoalStore struct {
	mu sync.RWMutex
	m  map[string]*Goal
}

// NewMemoryGoalStore returns an empty in-memory GoalStore.
func NewMemoryGoalStore() *MemoryGoalStore {
	return &MemoryGoalStore{m: make(map[string]*Goal)}
}

// Get returns a copy of the goal with the given id, or ErrGoalNotFound.
func (s *MemoryGoalStore) Get(_ context.Context, id string) (*Goal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.m[id]
	if !ok {
		return nil, ErrGoalNotFound
	}
	return cloneGoal(g), nil
}

// Upsert inserts or replaces the goal by id, updating UpdatedAt.
func (s *MemoryGoalStore) Upsert(_ context.Context, g *Goal) error {
	if g == nil || g.ID == "" {
		return errors.New("controlplane: goal id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	s.m[g.ID] = cloneGoal(g)
	return nil
}

// Delete removes the goal. A missing id is not an error.
func (s *MemoryGoalStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

// List returns shallow copies of all goals.
func (s *MemoryGoalStore) List(_ context.Context) ([]*Goal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Goal, 0, len(s.m))
	for _, g := range s.m {
		out = append(out, cloneGoal(g))
	}
	return out, nil
}
