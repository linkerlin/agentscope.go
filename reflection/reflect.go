package reflection

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// SelfReflectingAgent runs a writer agent and a critic agent in a loop.
// The writer produces a draft, the critic reviews it, and if the judge
// rejects the draft, the writer revises based on the critique.
// It implements agent.Agent.
type SelfReflectingAgent struct {
	name    string
	writer  agent.Agent
	critic  agent.Agent
	judge   func(draft, critique *message.Msg) bool
	maxIter int
}

// NewSelfReflectingAgent creates a reflection loop.
//   judge: returns true to accept the draft and stop iterating.
//   maxIter: hard upper bound (<=0 defaults to 3).
func NewSelfReflectingAgent(name string, writer, critic agent.Agent, judge func(draft, critique *message.Msg) bool, maxIter int) *SelfReflectingAgent {
	if maxIter <= 0 {
		maxIter = 3
	}
	return &SelfReflectingAgent{name: name, writer: writer, critic: critic, judge: judge, maxIter: maxIter}
}

func (s *SelfReflectingAgent) Name() string { return s.name }

func (s *SelfReflectingAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if s.writer == nil {
		return nil, fmt.Errorf("reflection %s: nil writer", s.name)
	}
	if s.critic == nil {
		return nil, fmt.Errorf("reflection %s: nil critic", s.name)
	}
	if s.judge == nil {
		return nil, fmt.Errorf("reflection %s: nil judge", s.name)
	}

	draft, err := s.writer.Call(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("reflection %s writer: %w", s.name, err)
	}

	for i := 0; i < s.maxIter; i++ {
		critique, err := s.critic.Call(ctx, draft)
		if err != nil {
			return nil, fmt.Errorf("reflection %s critic: %w", s.name, err)
		}
		if s.judge(draft, critique) {
			return draft, nil
		}
		revisionMsg := message.NewMsg().Role(message.RoleUser).TextContent(
			"Revise your previous output based on this feedback:\n"+critique.GetTextContent(),
		).Build()
		draft, err = s.writer.Call(ctx, revisionMsg)
		if err != nil {
			return nil, fmt.Errorf("reflection %s revision: %w", s.name, err)
		}
	}
	return draft, nil
}

// CallStream is not yet supported for reflection loops.
func (s *SelfReflectingAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, fmt.Errorf("reflection %s: streaming not supported", s.name)
}

var _ agent.Agent = (*SelfReflectingAgent)(nil)
