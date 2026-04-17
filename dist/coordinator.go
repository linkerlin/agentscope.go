package dist

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/linkerlin/agentscope.go/a2a"
)

// Strategy determines how a coordinator picks a remote agent.
type Strategy string

const (
	// Random picks a random agent from the registry.
	Random Strategy = "random"
	// RoundRobin cycles through agents in registration order.
	RoundRobin Strategy = "round_robin"
	// Broadcast sends to all agents and returns the first successful response.
	Broadcast Strategy = "broadcast"
)

// Coordinator dispatches A2A messages to remote agents using a Registry.
type Coordinator struct {
	registry  *Registry
	strategy  Strategy
	mu        sync.Mutex
	idx       int // round-robin cursor
	newClient func(baseURL string) a2a.Client
}

// NewCoordinator creates a dispatcher backed by the given registry.
func NewCoordinator(registry *Registry, strategy Strategy) *Coordinator {
	return &Coordinator{
		registry:  registry,
		strategy:  strategy,
		newClient: func(baseURL string) a2a.Client { return a2a.NewHTTPClient(baseURL) },
	}
}

// SetClientFactory overrides the default HTTP client factory (for testing).
func (c *Coordinator) SetClientFactory(fn func(baseURL string) a2a.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.newClient = fn
}

// Send delivers a message to a single remote agent according to the strategy.
func (c *Coordinator) Send(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
	cards := c.registry.List()
	if len(cards) == 0 {
		return nil, fmt.Errorf("dist: no agents available")
	}

	switch c.strategy {
	case Broadcast:
		return c.sendBroadcast(ctx, msg, cards)
	case RoundRobin:
		return c.sendRoundRobin(ctx, msg, cards)
	default:
		return c.sendRandom(ctx, msg, cards)
	}
}

func (c *Coordinator) sendRandom(ctx context.Context, msg *a2a.Message, cards []a2a.AgentCard) (*a2a.Message, error) {
	card := cards[rand.Intn(len(cards))]
	client := c.newClient(card.URL)
	return client.Send(ctx, msg)
}

func (c *Coordinator) sendRoundRobin(ctx context.Context, msg *a2a.Message, cards []a2a.AgentCard) (*a2a.Message, error) {
	c.mu.Lock()
	idx := c.idx % len(cards)
	c.idx++
	c.mu.Unlock()
	card := cards[idx]
	client := c.newClient(card.URL)
	return client.Send(ctx, msg)
}

func (c *Coordinator) sendBroadcast(ctx context.Context, msg *a2a.Message, cards []a2a.AgentCard) (*a2a.Message, error) {
	var wg sync.WaitGroup
	resultCh := make(chan *a2a.Message, 1)
	errCh := make(chan error, len(cards))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, card := range cards {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			client := c.newClient(url)
			resp, err := client.Send(ctx, msg)
			if err != nil {
				errCh <- err
				return
			}
			select {
			case resultCh <- resp:
				cancel() // first success stops the rest
			default:
			}
		}(card.URL)
	}

	go func() {
		wg.Wait()
		close(resultCh)
		close(errCh)
	}()

	if resp := <-resultCh; resp != nil {
		return resp, nil
	}
	// Collect at least one error
	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("dist: all broadcasts failed: %w", err)
	}
	return nil, fmt.Errorf("dist: all broadcasts failed")
}

// SendTo sends a message to a specific agent by URL.
func (c *Coordinator) SendTo(ctx context.Context, url string, msg *a2a.Message) (*a2a.Message, error) {
	card, ok := c.registry.Get(url)
	if !ok {
		return nil, fmt.Errorf("dist: agent %s not found in registry", url)
	}
	client := c.newClient(card.URL)
	return client.Send(ctx, msg)
}
