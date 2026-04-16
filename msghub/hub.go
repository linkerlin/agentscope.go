package msghub

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// Hub is a lightweight message hub for registering and dispatching
// messages to multiple agents.
type Hub struct {
	mu     sync.RWMutex
	agents map[string]agent.Agent
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{agents: make(map[string]agent.Agent)}
}

// Register adds an agent to the hub.
func (h *Hub) Register(name string, a agent.Agent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agents[name] = a
}

// Unregister removes an agent from the hub.
func (h *Hub) Unregister(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.agents, name)
}

// Names returns a snapshot of currently registered agent names.
func (h *Hub) Names() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.agents))
	for n := range h.agents {
		names = append(names, n)
	}
	return names
}

// Get retrieves a registered agent by name.
func (h *Hub) Get(name string) (agent.Agent, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	a, ok := h.agents[name]
	return a, ok
}

// Send dispatches a message to a single named agent.
func (h *Hub) Send(ctx context.Context, name string, msg *message.Msg) (*message.Msg, error) {
	a, ok := h.Get(name)
	if !ok {
		return nil, fmt.Errorf("msghub: agent %q not found", name)
	}
	return a.Call(ctx, msg)
}

// Broadcast sends the message to all registered agents concurrently.
// The returned map contains each agent's response (or an error message).
func (h *Hub) Broadcast(ctx context.Context, msg *message.Msg) map[string]*message.Msg {
	h.mu.RLock()
	agents := make(map[string]agent.Agent, len(h.agents))
	for k, v := range h.agents {
		agents[k] = v
	}
	h.mu.RUnlock()

	out := make(map[string]*message.Msg, len(agents))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, a := range agents {
		wg.Add(1)
		go func(n string, ag agent.Agent) {
			defer wg.Done()
			resp, err := ag.Call(ctx, msg)
			mu.Lock()
			if err != nil {
				out[n] = message.NewMsg().
					Role(message.RoleAssistant).
					TextContent(fmt.Sprintf("error: %v", err)).
					Build()
			} else {
				out[n] = resp
			}
			mu.Unlock()
		}(name, a)
	}
	wg.Wait()
	return out
}
