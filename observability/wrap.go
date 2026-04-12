package observability

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// TracedAgent 为 Agent 增加可选回调（不强制依赖 OpenTelemetry，便于渐进接入）
type TracedAgent struct {
	Inner    agent.Agent
	OnCall   func(ctx context.Context, name string, msg *message.Msg)
	OnResult func(ctx context.Context, name string, resp *message.Msg, err error)
	name     string
}

// NewTracedAgent 包装 Agent；name 用于回调标识
func NewTracedAgent(name string, inner agent.Agent) *TracedAgent {
	return &TracedAgent{name: name, Inner: inner}
}

func (t *TracedAgent) Name() string {
	if t.Inner != nil {
		return t.Inner.Name()
	}
	return t.name
}

func (t *TracedAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if t.OnCall != nil {
		t.OnCall(ctx, t.Name(), msg)
	}
	resp, err := t.Inner.Call(ctx, msg)
	if t.OnResult != nil {
		t.OnResult(ctx, t.Name(), resp, err)
	}
	return resp, err
}

func (t *TracedAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return t.Inner.CallStream(ctx, msg)
}

var _ agent.Agent = (*TracedAgent)(nil)
