package dist

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
)

// Registry maintains a set of remote agent cards discovered via A2A.
type Registry struct {
	mu    sync.RWMutex
	cards map[string]a2a.AgentCard // key = card.URL
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{cards: make(map[string]a2a.AgentCard)}
}

// Register adds or updates an agent card.
func (r *Registry) Register(card a2a.AgentCard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cards[card.URL] = card
}

// Deregister removes an agent by URL.
func (r *Registry) Deregister(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cards, url)
}

// Get returns a card by URL.
func (r *Registry) Get(url string) (a2a.AgentCard, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.cards[url]
	return c, ok
}

// List returns all registered cards.
func (r *Registry) List() []a2a.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]a2a.AgentCard, 0, len(r.cards))
	for _, c := range r.cards {
		out = append(out, c)
	}
	return out
}

// Discover fetches an AgentCard from a remote agent's well-known endpoint.
func Discover(ctx context.Context, baseURL string, httpClient *http.Client) (a2a.AgentCard, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/.well-known/agent.json", nil)
	if err != nil {
		return a2a.AgentCard{}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return a2a.AgentCard{}, fmt.Errorf("discover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return a2a.AgentCard{}, fmt.Errorf("discover: status %d", resp.StatusCode)
	}
	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return a2a.AgentCard{}, fmt.Errorf("discover: %w", err)
	}
	return card, nil
}

// AutoDiscover periodically polls a list of base URLs and keeps the registry up-to-date.
type AutoDiscover struct {
	registry   *Registry
	urls       []string
	interval   time.Duration
	httpClient *http.Client
	cancel     context.CancelFunc
}

// NewAutoDiscover creates a background discovery worker.
func NewAutoDiscover(registry *Registry, urls []string, interval time.Duration, httpClient *http.Client) *AutoDiscover {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &AutoDiscover{
		registry:   registry,
		urls:       urls,
		interval:   interval,
		httpClient: httpClient,
	}
}

// Start begins the background discovery loop.
func (ad *AutoDiscover) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	ad.cancel = cancel
	go ad.loop(ctx)
}

// Stop halts the background discovery loop.
func (ad *AutoDiscover) Stop() {
	if ad.cancel != nil {
		ad.cancel()
	}
}

func (ad *AutoDiscover) loop(ctx context.Context) {
	ticker := time.NewTicker(ad.interval)
	defer ticker.Stop()
	ad.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ad.runOnce(ctx)
		}
	}
}

func (ad *AutoDiscover) runOnce(ctx context.Context) {
	for _, u := range ad.urls {
		card, err := Discover(ctx, u, ad.httpClient)
		if err != nil {
			continue
		}
		ad.registry.Register(card)
	}
}
