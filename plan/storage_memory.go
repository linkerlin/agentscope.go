package plan

import (
	"fmt"
	"sync"
)

// InMemoryStorage is a thread-safe in-memory implementation of Storage.
type InMemoryStorage struct {
	mu     sync.RWMutex
	plans  map[string]*Plan
}

// NewInMemoryStorage creates a new in-memory plan storage.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{plans: make(map[string]*Plan)}
}

// AddPlan stores a plan (replaces if same ID exists).
func (s *InMemoryStorage) AddPlan(p *Plan) error {
	if p == nil {
		return fmt.Errorf("plan: nil plan")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans[p.ID] = p
	return nil
}

// GetPlan retrieves a plan by ID.
func (s *InMemoryStorage) GetPlan(planID string) (*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	return p, nil
}

// ListPlans returns all stored plans.
func (s *InMemoryStorage) ListPlans() ([]*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Plan, 0, len(s.plans))
	for _, p := range s.plans {
		out = append(out, p)
	}
	return out, nil
}

// DeletePlan removes a plan by ID.
func (s *InMemoryStorage) DeletePlan(planID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.plans, planID)
	return nil
}
