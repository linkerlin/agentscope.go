package observability

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// TracedAgent 为 Agent 增加可选回调与轻量 Tracer 集成（不强制依赖 OpenTelemetry）
type TracedAgent struct {
	Inner    agent.Agent
	OnCall   func(ctx context.Context, name string, msg *message.Msg)
	OnResult func(ctx context.Context, name string, resp *message.Msg, err error)
	Tracer   Tracer
	name     string
}

// NewTracedAgent 包装 Agent；name 用于回调标识
func NewTracedAgent(name string, inner agent.Agent) *TracedAgent {
	return &TracedAgent{name: name, Inner: inner, Tracer: NoopTracer}
}

// WithTracer 设置内部 Tracer（用于创建调用 span）
func (t *TracedAgent) WithTracer(tracer Tracer) *TracedAgent {
	if tracer != nil {
		t.Tracer = tracer
	}
	return t
}

func (t *TracedAgent) Name() string {
	if t.Inner != nil {
		return t.Inner.Name()
	}
	return t.name
}

func (t *TracedAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name()+"_call")
	defer span.End()

	if t.OnCall != nil {
		t.OnCall(ctx, t.Name(), msg)
	}
	resp, err := t.Inner.Call(ctx, msg)
	if err != nil {
		span.RecordError(err)
	}
	if t.OnResult != nil {
		t.OnResult(ctx, t.Name(), resp, err)
	}
	return resp, err
}

func (t *TracedAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name()+"_call_stream")
	defer span.End()

	if t.OnCall != nil {
		t.OnCall(ctx, t.Name(), msg)
	}
	ch, err := t.Inner.CallStream(ctx, msg)
	if err != nil {
		span.RecordError(err)
	}
	return ch, err
}

var _ agent.Agent = (*TracedAgent)(nil)
