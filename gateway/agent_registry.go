package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/service"
)

// AgentRegistry stores and retrieves named agent instances.
// It supports both static registration (pre-built agents) and dynamic
// creation via AgentFactory from persisted AgentConfig + Credential.
type AgentRegistry struct {
	agents   map[string]agent.Agent // id -> agent
	factory  *AgentFactory
	storage  service.Storage
	mu       sync.RWMutex
}

// NewAgentRegistry creates an empty registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]agent.Agent),
	}
}

// WithFactory attaches an AgentFactory for dynamic agent creation.
func (r *AgentRegistry) WithFactory(f *AgentFactory) *AgentRegistry {
	r.factory = f
	return r
}

// WithStorage attaches a Storage so that Get can lazily load configs.
func (r *AgentRegistry) WithStorage(st service.Storage) *AgentRegistry {
	r.storage = st
	return r
}

// Register adds a pre-built agent under the given ID.
func (r *AgentRegistry) Register(id string, a agent.Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[id] = a
}

// Get retrieves an agent by ID.
//
// Resolution order:
//  1. Static registry lookup.
//  2. If not found and storage + factory are available, load AgentConfig
//     and Credential from storage and build the agent on demand.
//  3. Return error if none of the above succeed.
func (r *AgentRegistry) Get(ctx context.Context, id string) (agent.Agent, error) {
	r.mu.RLock()
	a, ok := r.agents[id]
	r.mu.RUnlock()
	if ok {
		return a, nil
	}

	if r.storage == nil || r.factory == nil {
		return nil, fmt.Errorf("agent_registry: agent %q not found", id)
	}

	// Lazy load from storage.
	cfg, err := r.storage.GetAgentConfig(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent_registry: load config for %q: %w", id, err)
	}

	creds, err := r.storage.ListCredentialsByUser(ctx, cfg.UserID)
	if err != nil {
		return nil, fmt.Errorf("agent_registry: load credentials for %q: %w", id, err)
	}

	var matched *service.Credential
	for _, c := range creds {
		if c.Provider == providerFromModelID(cfg.ModelID) {
			matched = c
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("agent_registry: no credential for provider %q", providerFromModelID(cfg.ModelID))
	}

	a, err = r.factory.Build(cfg, matched)
	if err != nil {
		return nil, fmt.Errorf("agent_registry: build agent %q: %w", id, err)
	}

	// Cache the built agent.
	r.mu.Lock()
	r.agents[id] = a
	r.mu.Unlock()
	return a, nil
}

// Remove evicts an agent from the registry.
func (r *AgentRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, id)
}

// Len returns the number of statically registered agents.
func (r *AgentRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// providerFromModelID extracts the provider prefix from a model ID.
func providerFromModelID(modelID string) string {
	provider, _ := parseModelID(modelID, "")
	return provider
}
