package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RegistryEntry wraps an AgentCard with discovery metadata.
type RegistryEntry struct {
	Card         AgentCard `json:"card"`
	DiscoveredAt time.Time `json:"discovered_at"`
	LastSeen     time.Time `json:"last_seen"`
	Healthy      bool      `json:"healthy"`
}

// Registry maintains a dynamic set of discoverable A2A agents.
// It delegates persistence to a RegistryStore so that the registry can be
// backed by an in-memory map, Redis, or any other shared storage.
type Registry struct {
	store    RegistryStore
	client   *http.Client
	watchers watcherSet
}

// NewRegistry creates a new A2A agent registry with an in-memory store.
func NewRegistry() *Registry {
	return NewRegistryWithStore(newInMemoryRegistryStore())
}

// NewRegistryWithStore creates a registry backed by the given store.
func NewRegistryWithStore(store RegistryStore) *Registry {
	return &Registry{
		store:  store,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register manually registers an AgentCard.
func (r *Registry) Register(card AgentCard) error {
	if card.URL == "" {
		return fmt.Errorf("a2a registry: card URL is required")
	}
	now := time.Now()
	if err := r.store.Register(context.Background(), RegistryEntry{
		Card:         card,
		DiscoveredAt: now,
		LastSeen:     now,
		Healthy:      true,
	}); err != nil {
		return err
	}
	r.watchers.notify(RegistryChange{URL: card.URL, Healthy: true, Op: ChangeOpRegister})
	return nil
}

// Discover fetches the AgentCard from a remote URL and registers it.
func (r *Registry) Discover(ctx context.Context, agentURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, agentURL+"/.well-known/agent.json", nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("a2a registry: %s", resp.Status)
	}
	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return err
	}
	card.URL = agentURL
	return r.Register(card)
}

// List returns all registered entries.
func (r *Registry) List() []RegistryEntry {
	entries, err := r.store.List(context.Background())
	if err != nil {
		return nil
	}
	return entries
}

// Get looks up an entry by URL.
func (r *Registry) Get(url string) (*RegistryEntry, bool) {
	entry, err := r.store.Get(context.Background(), url)
	if err != nil || entry == nil {
		return nil, false
	}
	return entry, true
}

// HealthCheck probes all registered agents and updates their health status.
// It emits ChangeOpHealth events when an agent's health status changes.
func (r *Registry) HealthCheck(ctx context.Context) {
	entries, err := r.store.List(ctx)
	if err != nil {
		return
	}
	for _, e := range entries {
		previous := e.Healthy
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.Card.URL+"/.well-known/agent.json", nil)
		resp, err := r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			_ = r.store.UpdateHealth(ctx, e.Card.URL, false, e.LastSeen)
			if previous {
				r.watchers.notify(RegistryChange{URL: e.Card.URL, Healthy: false, Op: ChangeOpHealth})
			}
		} else {
			_ = r.store.UpdateHealth(ctx, e.Card.URL, true, time.Now())
			resp.Body.Close()
			if !previous {
				r.watchers.notify(RegistryChange{URL: e.Card.URL, Healthy: true, Op: ChangeOpHealth})
			}
		}
	}
}

// Remove unregisters an agent.
func (r *Registry) Remove(url string) {
	if err := r.store.Remove(context.Background(), url); err == nil {
		r.watchers.notify(RegistryChange{URL: url, Op: ChangeOpRemove})
	}
}

// StartBackgroundHealthCheck starts a goroutine that runs HealthCheck at the
// given interval. The goroutine stops when the context is cancelled.
// Returns a function that can be called to stop the background checker.
func (r *Registry) StartBackgroundHealthCheck(ctx context.Context, interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				r.HealthCheck(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { close(stop) }
}
