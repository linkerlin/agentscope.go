package a2atool

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/a2a"
	"github.com/linkerlin/agentscope.go/tool"
)

// RemoteAgent describes a registered A2A endpoint.
type RemoteAgent struct {
	Name        string
	Description string
	URL         string
}

// ClientFactory creates an a2a.Client for a given URL.
type ClientFactory func(url string) a2a.Client

// Registry manages a set of remote A2A agents and produces A2ATool instances.
type Registry struct {
	mu      sync.RWMutex
	agents  map[string]*RemoteAgent
	factory ClientFactory
	cache   map[string]a2a.Client
}

// NewRegistry creates a Registry with the given client factory.
// If factory is nil, a2a.NewHTTPClient is used.
func NewRegistry(factory ClientFactory) *Registry {
	if factory == nil {
		factory = func(url string) a2a.Client {
			return a2a.NewHTTPClient(url)
		}
	}
	return &Registry{
		agents:  make(map[string]*RemoteAgent),
		factory: factory,
		cache:   make(map[string]a2a.Client),
	}
}

// Register adds a remote agent to the registry.
func (r *Registry) Register(name, description, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[name] = &RemoteAgent{Name: name, Description: description, URL: url}
}

// RegisterFromAgentCards adds all agents from the given AgentCards.
func (r *Registry) RegisterFromAgentCards(cards []a2a.AgentCard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range cards {
		r.agents[c.Name] = &RemoteAgent{
			Name:        c.Name,
			Description: c.Description,
			URL:         c.URL,
		}
	}
}

// Get retrieves a registered remote agent by name.
func (r *Registry) Get(name string) (*RemoteAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ra, ok := r.agents[name]
	return ra, ok
}

// getClient returns (and caches) an a2a.Client for the given agent name.
func (r *Registry) getClient(name string) (a2a.Client, error) {
	r.mu.RLock()
	ra, ok := r.agents[name]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("a2a agent %q not registered", name)
	}
	if c, cached := r.cache[name]; cached {
		r.mu.RUnlock()
		return c, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, cached := r.cache[name]; cached {
		return c, nil
	}
	c := r.factory(ra.URL)
	r.cache[name] = c
	return c, nil
}

// CreateTool builds an A2ATool for the named remote agent.
func (r *Registry) CreateTool(name string) (*A2ATool, error) {
	ra, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("a2a agent %q not registered", name)
	}
	client, err := r.getClient(name)
	if err != nil {
		return nil, err
	}
	return NewA2ATool(ra.Name, ra.Description, client), nil
}

// AllTools returns A2ATool instances for every registered agent.
func (r *Registry) AllTools() []tool.Tool {
	r.mu.RLock()
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	r.mu.RUnlock()

	tools := make([]tool.Tool, 0, len(names))
	for _, name := range names {
		t, err := r.CreateTool(name)
		if err == nil {
			tools = append(tools, t)
		}
	}
	return tools
}

// Close closes all cached clients.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	for _, c := range r.cache {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.cache = make(map[string]a2a.Client)
	return firstErr
}

// Ping checks if a remote agent is reachable by sending a minimal message.
func (r *Registry) Ping(ctx context.Context, name string) error {
	client, err := r.getClient(name)
	if err != nil {
		return err
	}
	_, err = client.Send(ctx, &a2a.Message{Role: "user", Content: "ping"})
	return err
}
