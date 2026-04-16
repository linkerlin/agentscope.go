package pipeline

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// Pipeline chains multiple agents sequentially.
// The output of step i becomes the input of step i+1.
// It implements agent.Agent, so it can be used anywhere an agent is expected.
type Pipeline struct {
	name  string
	steps []agent.Agent
}

// New creates a Pipeline with the given name and ordered steps.
func New(name string, steps ...agent.Agent) *Pipeline {
	return &Pipeline{name: name, steps: steps}
}

// Name returns the pipeline name.
func (p *Pipeline) Name() string { return p.name }

// Call executes each step in order.
// The input message is passed to the first step; each subsequent step receives
// the previous step's output as a user message.
func (p *Pipeline) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if len(p.steps) == 0 {
		return nil, fmt.Errorf("pipeline %s: no steps", p.name)
	}
	current := msg
	for i, step := range p.steps {
		out, err := step.Call(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("pipeline %s step %d (%s): %w", p.name, i, step.Name(), err)
		}
		current = message.NewMsg().Role(message.RoleUser).TextContent(out.GetTextContent()).Build()
	}
	// Return the final output with assistant role for natural consumption.
	return message.NewMsg().Role(message.RoleAssistant).TextContent(current.GetTextContent()).Build(), nil
}

// CallStream is not yet supported for pipelines.
func (p *Pipeline) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("pipeline %s: streaming not supported", p.name)
}

var _ agent.Agent = (*Pipeline)(nil)
