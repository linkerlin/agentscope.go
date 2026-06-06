package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// Parallel runs multiple agents concurrently and aggregates their outputs.
// It implements agent.Agent, so it can be nested inside Pipelines or used
// as a step in other orchestration patterns.
type Parallel struct {
	name string
	// Aggregator combines the individual agent outputs into a single message.
	// If nil, the default aggregator joins non-empty text results with "\n---\n".
	Aggregator func([]*message.Msg) *message.Msg
	steps      []agent.Agent
}

// NewParallel creates a Parallel pipeline with the given name and steps.
func NewParallel(name string, steps ...agent.Agent) *Parallel {
	return &Parallel{name: name, steps: steps}
}

// WithAggregator sets a custom aggregation function.
func (p *Parallel) WithAggregator(fn func([]*message.Msg) *message.Msg) *Parallel {
	p.Aggregator = fn
	return p
}

// Name returns the pipeline name.
func (p *Parallel) Name() string { return p.name }

// Call executes all steps in parallel, waits for completion, and aggregates results.
func (p *Parallel) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if len(p.steps) == 0 {
		return nil, fmt.Errorf("parallel %s: no steps", p.name)
	}

	type result struct {
		idx int
		msg *message.Msg
		err error
	}

	results := make([]result, len(p.steps))
	var wg sync.WaitGroup
	for i, step := range p.steps {
		wg.Add(1)
		go func(idx int, a agent.Agent) {
			defer wg.Done()
			m, err := a.Call(ctx, msg)
			results[idx] = result{idx: idx, msg: m, err: err}
		}(i, step)
	}
	wg.Wait()

	// Check errors — fail fast if any step errors.
	for _, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("parallel %s step %d (%s): %w", p.name, r.idx, p.steps[r.idx].Name(), r.err)
		}
	}

	msgs := make([]*message.Msg, len(results))
	for i, r := range results {
		msgs[i] = r.msg
	}

	if p.Aggregator != nil {
		return p.Aggregator(msgs), nil
	}
	return defaultAggregate(msgs), nil
}

// CallStream is not yet supported for parallel pipelines.
func (p *Parallel) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("parallel %s: streaming not supported", p.name)
}

func defaultAggregate(msgs []*message.Msg) *message.Msg {
	var parts []string
	for _, m := range msgs {
		if m == nil {
			continue
		}
		txt := m.GetTextContent()
		if txt != "" {
			parts = append(parts, txt)
		}
	}
	return message.NewMsg().
		Role(message.RoleAssistant).
		TextContent(strings.Join(parts, "\n---\n")).
		Build()
}

var _ agent.Agent = (*Parallel)(nil)
