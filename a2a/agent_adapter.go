package a2a

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// AgentAdapter wraps an agent.Agent to implement AgentRunner and StreamingAgentRunner.
type AgentAdapter struct {
	agent agent.Agent
}

// NewAgentAdapter creates a new adapter for the given agent.
func NewAgentAdapter(a agent.Agent) *AgentAdapter {
	return &AgentAdapter{agent: a}
}

// toMsg converts an A2A Message to a message.Msg.
func toMsg(m *Message) *message.Msg {
	return message.NewMsg().
		Role(message.RoleUser).
		TextContent(m.Content).
		Build()
}

// fromMsg converts a message.Msg to an A2A Message.
func fromMsg(m *message.Msg) *Message {
	role := string(m.Role)
	if m.Role == message.RoleAssistant {
		role = "agent"
	}
	return &Message{
		Role:    role,
		Content: m.GetTextContent(),
	}
}

// Run implements AgentRunner.
func (a *AgentAdapter) Run(ctx context.Context, msg *Message) (*Message, error) {
	resp, err := a.agent.Call(ctx, toMsg(msg))
	if err != nil {
		return nil, err
	}
	return fromMsg(resp), nil
}

// RunStream implements StreamingAgentRunner.
func (a *AgentAdapter) RunStream(ctx context.Context, msg *Message) (<-chan *Message, error) {
	ch, err := a.agent.CallStream(ctx, toMsg(msg))
	if err != nil {
		return nil, err
	}

	out := make(chan *Message)
	go func() {
		defer close(out)
		for m := range ch {
			if m == nil {
				continue
			}
			out <- fromMsg(m)
		}
	}()
	return out, nil
}

var _ AgentRunner = (*AgentAdapter)(nil)
var _ StreamingAgentRunner = (*AgentAdapter)(nil)
