package workflow

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// Loop repeatedly executes a body agent while a condition holds true.
// It implements agent.Agent.
type Loop struct {
	name      string
	body      agent.Agent
	condition func(*message.Msg) bool
	maxIter   int
}

// NewLoop creates a loop workflow step.
//   condition: returns true to continue looping.
//   maxIter:   hard upper bound to prevent infinite loops (<=0 defaults to 10).
func NewLoop(name string, body agent.Agent, condition func(*message.Msg) bool, maxIter int) *Loop {
	if maxIter <= 0 {
		maxIter = 10
	}
	return &Loop{name: name, body: body, condition: condition, maxIter: maxIter}
}

func (l *Loop) Name() string { return l.name }

func (l *Loop) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if l.body == nil {
		return nil, fmt.Errorf("workflow %s: nil body", l.name)
	}
	if l.condition == nil {
		return nil, fmt.Errorf("workflow %s: nil condition", l.name)
	}
	current := msg
	for i := 0; i < l.maxIter; i++ {
		out, err := l.body.Call(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("workflow %s iteration %d: %w", l.name, i, err)
		}
		current = out
		if !l.condition(current) {
			break
		}
	}
	return current, nil
}

// CallStream is not yet supported for loop execution.
func (l *Loop) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("workflow %s: streaming not supported", l.name)
}

var _ agent.Agent = (*Loop)(nil)
