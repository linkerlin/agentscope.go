package a2a

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RegistryStore abstracts the persistence layer for A2A registry entries.
// Implementations may be in-memory, Redis, or any other shared store.
type RegistryStore interface {
	// Register persists a registry entry. The entry's Card.URL is the primary key.
	Register(ctx context.Context, entry RegistryEntry) error
	// Get returns a single entry by card URL.
	Get(ctx context.Context, url string) (*RegistryEntry, error)
	// List returns all known entries.
	List(ctx context.Context) ([]RegistryEntry, error)
	// Remove deletes the entry identified by url.
	Remove(ctx context.Context, url string) error
	// UpdateHealth updates the health flag and last-seen timestamp for an entry.
	UpdateHealth(ctx context.Context, url string, healthy bool, lastSeen time.Time) error
}

// inMemoryRegistryStore is the default in-process RegistryStore.
type inMemoryRegistryStore struct {
	mu      sync.RWMutex
	entries map[string]*RegistryEntry
}

func newInMemoryRegistryStore() *inMemoryRegistryStore {
	return &inMemoryRegistryStore{entries: make(map[string]*RegistryEntry)}
}

func (s *inMemoryRegistryStore) Register(_ context.Context, entry RegistryEntry) error {
	if entry.Card.URL == "" {
		return fmt.Errorf("a2a registry: card URL is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.Card.URL] = &entry
	return nil
}

func (s *inMemoryRegistryStore) Get(_ context.Context, url string) (*RegistryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.entries[url]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, nil
}

func (s *inMemoryRegistryStore) List(_ context.Context) ([]RegistryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RegistryEntry, 0, len(s.entries))
	for _, e := range s.entries {
		cp := *e
		out = append(out, cp)
	}
	return out, nil
}

func (s *inMemoryRegistryStore) Remove(_ context.Context, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, url)
	return nil
}

func (s *inMemoryRegistryStore) UpdateHealth(_ context.Context, url string, healthy bool, lastSeen time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[url]; ok {
		e.Healthy = healthy
		e.LastSeen = lastSeen
	}
	return nil
}

var _ RegistryStore = (*inMemoryRegistryStore)(nil)
