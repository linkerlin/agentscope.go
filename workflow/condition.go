package workflow

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// Condition branches to one of two agents based on an evaluator function.
// It implements agent.Agent.
type Condition struct {
	name      string
	evaluator func(*message.Msg) bool
	ifTrue    agent.Agent
	ifFalse   agent.Agent
}

// NewCondition creates a conditional workflow step.
func NewCondition(name string, evaluator func(*message.Msg) bool, ifTrue, ifFalse agent.Agent) *Condition {
	return &Condition{name: name, evaluator: evaluator, ifTrue: ifTrue, ifFalse: ifFalse}
}

func (c *Condition) Name() string { return c.name }

func (c *Condition) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if c.evaluator == nil {
		return nil, fmt.Errorf("workflow %s: nil evaluator", c.name)
	}
	if c.evaluator(msg) {
		if c.ifTrue == nil {
			return nil, fmt.Errorf("workflow %s: nil ifTrue branch", c.name)
		}
		return c.ifTrue.Call(ctx, msg)
	}
	if c.ifFalse == nil {
		return nil, fmt.Errorf("workflow %s: nil ifFalse branch", c.name)
	}
	return c.ifFalse.Call(ctx, msg)
}

// CallStream is not yet supported for conditional execution.
func (c *Condition) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("workflow %s: streaming not supported", c.name)
}

var _ agent.Agent = (*Condition)(nil)
