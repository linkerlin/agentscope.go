package controlplane

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Todo is a structured checkbox inside a goal, addressed by TodoID. There is
// no separate "issue" object: the goal boundary (GoalID) plus the todo id is
// the full address. A todo may carry soft ownership (ClaimedBy) and, later, a
// hard lease over CoordBus (P2).
//
// Supersession lineage is encoded as data: when a todo is replaced by a
// successor, SupersededBy names the successor and the successor's Supersedes
// names this todo. A projection renders that lineage without reading private
// docs (LoopX "row lineage as data" discipline).
type Todo struct {
	ID           string             `json:"id"`
	GoalID       string             `json:"goal_id"`
	OwnerUserID  string             `json:"owner_user_id,omitempty"` // tenant scope (#5); empty = legacy/global
	Description  string             `json:"description"`
	TaskClass    TaskClass          `json:"task_class"`
	StageID      string             `json:"stage_id,omitempty"` // capability lane stage, e.g. "review"/"merge" (#3c)
	State        TodoState          `json:"state"`
	ClaimedBy    string             `json:"claimed_by,omitempty"`
	Continuation ContinuationPolicy `json:"continuation,omitempty"`
	Order        int                `json:"order,omitempty"`
	// Evidence holds the full evidence artifacts (id/kind/summary/source_ref)
	// backing this todo — the single source of truth (#5 round-5). EvidenceIDs
	// is DERIVED from it; the old parallel field was removed to prevent drift.
	Evidence []Evidence `json:"evidence,omitempty"`
	// Supersedes is the predecessor todo id this todo replaced (if any).
	Supersedes string `json:"supersedes,omitempty"`
	// SupersededBy is the successor todo id that replaced this todo (if any).
	SupersededBy string    `json:"superseded_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// todoTransitions is the legal todo state-transition table. Only open may
// move; blocked may return to open. Terminal states (done/deferred) freeze.
var todoTransitions = map[TodoState]map[TodoState]bool{
	TodoOpen: {
		TodoDone:     true,
		TodoBlocked:  true,
		TodoDeferred: true,
	},
	TodoBlocked: {
		TodoOpen:     true,
		TodoDeferred: true,
	},
	TodoDone:     {},
	TodoDeferred: {},
}

// LegalTodoTransition reports whether moving from -> to is permitted.
func LegalTodoTransition(from, to TodoState) bool {
	if to == from {
		return true
	}
	return todoTransitions[from][to]
}

// EvidenceIDs derives the evidence id list from Evidence (#5 round-5). The old
// parallel field could drift; Evidence is the single source of truth.
func (t *Todo) EvidenceIDs() []string {
	ids := make([]string, 0, len(t.Evidence))
	for _, e := range t.Evidence {
		ids = append(ids, e.ID)
	}
	return ids
}

// ErrTodoNotFound is returned by TodoStore lookups for a missing todo.
var ErrTodoNotFound = errors.New("controlplane: todo not found")

// TodoStore persists Todo state scoped to a goal.
type TodoStore interface {
	Get(ctx context.Context, goalID, todoID string) (*Todo, error)
	List(ctx context.Context, goalID string) ([]*Todo, error)
	Upsert(ctx context.Context, t *Todo) error
	Delete(ctx context.Context, goalID, todoID string) error
}

// MemoryTodoStore is a concurrency-safe in-process TodoStore.
type MemoryTodoStore struct {
	mu sync.RWMutex
	m  map[string]map[string]*Todo // goalID -> todoID -> todo
}

// NewMemoryTodoStore returns an empty in-memory TodoStore.
func NewMemoryTodoStore() *MemoryTodoStore {
	return &MemoryTodoStore{m: make(map[string]map[string]*Todo)}
}

// Get returns a copy of the todo, or ErrTodoNotFound.
func (s *MemoryTodoStore) Get(_ context.Context, goalID, todoID string) (*Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.m[goalID]
	if !ok {
		return nil, ErrTodoNotFound
	}
	t, ok := g[todoID]
	if !ok {
		return nil, ErrTodoNotFound
	}
	return cloneTodo(t), nil
}

// List returns shallow copies of all todos for a goal, ordered by Order then ID.
func (s *MemoryTodoStore) List(_ context.Context, goalID string) ([]*Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.m[goalID]
	if !ok {
		return []*Todo{}, nil
	}
	out := make([]*Todo, 0, len(g))
	for _, t := range g {
		out = append(out, cloneTodo(t))
	}
	return out, nil
}

// Upsert inserts or replaces the todo, keyed by (GoalID, ID).
func (s *MemoryTodoStore) Upsert(_ context.Context, t *Todo) error {
	if t == nil || t.ID == "" || t.GoalID == "" {
		return errors.New("controlplane: todo id and goal_id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if s.m[t.GoalID] == nil {
		s.m[t.GoalID] = make(map[string]*Todo)
	}
	s.m[t.GoalID][t.ID] = cloneTodo(t)
	return nil
}

// Delete removes a todo. A missing id is not an error.
func (s *MemoryTodoStore) Delete(_ context.Context, goalID, todoID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.m[goalID]; ok {
		delete(g, todoID)
	}
	return nil
}
