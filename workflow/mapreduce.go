package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// MapReduce splits an input into chunks, maps each chunk through an agent
// in parallel (with bounded concurrency), and then reduces the mapped
// outputs into a final result.
// It implements agent.Agent.
type MapReduce struct {
	name        string
	split       func(*message.Msg) []string
	mapper      agent.Agent
	reducer     agent.Agent
	parallelism int
}

// NewMapReduce creates a MapReduce workflow step.
//   split:       splits the input message into independent text chunks.
//   mapper:      agent that processes each chunk.
//   reducer:     agent that combines all mapped outputs into the final result.
//   parallelism: max concurrent mapper calls (<=0 defaults to 4).
func NewMapReduce(name string, split func(*message.Msg) []string, mapper, reducer agent.Agent, parallelism int) *MapReduce {
	if parallelism <= 0 {
		parallelism = 4
	}
	return &MapReduce{name: name, split: split, mapper: mapper, reducer: reducer, parallelism: parallelism}
}

func (m *MapReduce) Name() string { return m.name }

func (m *MapReduce) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.split == nil {
		return nil, fmt.Errorf("mapreduce %s: nil split", m.name)
	}
	if m.mapper == nil {
		return nil, fmt.Errorf("mapreduce %s: nil mapper", m.name)
	}
	if m.reducer == nil {
		return nil, fmt.Errorf("mapreduce %s: nil reducer", m.name)
	}

	chunks := m.split(msg)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("mapreduce %s: no chunks produced", m.name)
	}

	// Map phase with bounded parallelism.
	sem := make(chan struct{}, m.parallelism)
	mapped := make([]*message.Msg, len(chunks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, text string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resp, err := m.mapper.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent(text).Build())
			mu.Lock()
			if err != nil {
				mapped[idx] = message.NewMsg().
					Role(message.RoleAssistant).
					TextContent(fmt.Sprintf("error: %v", err)).
					Build()
			} else {
				mapped[idx] = resp
			}
			mu.Unlock()
		}(i, chunk)
	}
	wg.Wait()

	// Reduce phase.
	var parts []string
	for _, r := range mapped {
		if r != nil {
			parts = append(parts, r.GetTextContent())
		}
	}
	reduceInput := message.NewMsg().
		Role(message.RoleUser).
		TextContent(strings.Join(parts, "\n---\n")).
		Build()
	return m.reducer.Call(ctx, reduceInput)
}

// CallStream is not yet supported for MapReduce.
func (m *MapReduce) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("mapreduce %s: streaming not supported", m.name)
}

var _ agent.Agent = (*MapReduce)(nil)
