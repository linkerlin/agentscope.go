package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// Parallel runs multiple agents concurrently and merges their outputs.
// It implements agent.Agent, so it can be nested inside Pipeline or other workflows.
type Parallel struct {
	name  string
	items []agent.Agent
	join  func([]*message.Msg) *message.Msg
}

// NewParallel creates a parallel step. If join is nil, a default text-join is used.
func NewParallel(name string, join func([]*message.Msg) *message.Msg, items ...agent.Agent) *Parallel {
	if join == nil {
		join = defaultTextJoin
	}
	return &Parallel{name: name, items: items, join: join}
}

func (p *Parallel) Name() string { return p.name }

func (p *Parallel) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if len(p.items) == 0 {
		return nil, fmt.Errorf("workflow %s: no parallel items", p.name)
	}
	results := make([]*message.Msg, len(p.items))
	var wg sync.WaitGroup
	for i, a := range p.items {
		wg.Add(1)
		go func(idx int, ag agent.Agent) {
			defer wg.Done()
			resp, err := ag.Call(ctx, msg)
			if err != nil {
				resp = message.NewMsg().
					Role(message.RoleAssistant).
					TextContent(fmt.Sprintf("error from %s: %v", ag.Name(), err)).
					Build()
			}
			results[idx] = resp
		}(i, a)
	}
	wg.Wait()
	return p.join(results), nil
}

// CallStream is not yet supported for parallel execution.
func (p *Parallel) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("workflow %s: streaming not supported", p.name)
}

func defaultTextJoin(msgs []*message.Msg) *message.Msg {
	var parts []string
	for _, m := range msgs {
		if m != nil {
			parts = append(parts, m.GetTextContent())
		}
	}
	return message.NewMsg().
		Role(message.RoleAssistant).
		TextContent(strings.Join(parts, "\n")).
		Build()
}

var _ agent.Agent = (*Parallel)(nil)
