package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*RegistryEntry // key = card.URL
	client  *http.Client
}

// NewRegistry creates a new A2A agent registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]*RegistryEntry),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Register manually registers an AgentCard.
func (r *Registry) Register(card AgentCard) error {
	if card.URL == "" {
		return fmt.Errorf("a2a registry: card URL is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[card.URL] = &RegistryEntry{
		Card:         card,
		DiscoveredAt: time.Now(),
		LastSeen:     time.Now(),
		Healthy:      true,
	}
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RegistryEntry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, *e)
	}
	return out
}

// Get looks up an entry by URL.
func (r *Registry) Get(url string) (*RegistryEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[url]
	return e, ok
}

// HealthCheck probes all registered agents and updates their health status.
func (r *Registry) HealthCheck(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for url, e := range r.entries {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/.well-known/agent.json", nil)
		resp, err := r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			e.Healthy = false
		} else {
			e.Healthy = true
			e.LastSeen = time.Now()
			resp.Body.Close()
		}
		r.entries[url] = e
	}
}

// Remove unregisters an agent.
func (r *Registry) Remove(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, url)
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
