package observability

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/tool"
)

// TracingMiddlewareAdapter 提供一个可作为 middleware 使用的 tracing 包装（避免循环依赖）。
// 用户可在 agent/react builder 或 toolkit 中使用。
// 参考 Python middleware/_tracing/ 对齐。
//
// 使用方式示例 (在 middleware 链中)：
//
//	adapter := &observability.TracingMiddlewareAdapter{Tracer: myTracer, Name: "myagent"}
//	// 然后实现 ReplyInterceptor 等 by 委托到 adapter.Tracer
//	// 或使用 TracedAgent 包装 agent。
type TracingMiddlewareAdapter struct {
	middleware.Base // embed to satisfy Middleware interface (provides unexported middleware() method)
	Tracer          Tracer
	Name            string
}

func (t *TracingMiddlewareAdapter) OnCall(ctx context.Context, name string, msg *message.Msg) {
	if t.Tracer != nil {
		_, span := t.Tracer.Start(ctx, t.Name+"_"+name)
		span.End()
	}
}

func (t *TracingMiddlewareAdapter) OnResult(ctx context.Context, name string, resp *message.Msg, err error) {
	// 可扩展记录
}

// Implement middleware interfaces for use in agent middleware chain (on_reply etc.)
// This allows direct use like: builder.Middlewares(&observability.TracingMiddlewareAdapter{...})

func (t *TracingMiddlewareAdapter) OnReply(ctx context.Context, agent middleware.Agent, input *middleware.ReplyInput, next middleware.ReplyNext) (*message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_reply")
	defer span.End()
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnReasoning(ctx context.Context, agent middleware.Agent, input *middleware.ReasoningInput, next middleware.ReasoningNext) (*message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_reasoning")
	defer span.End()
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnActing(ctx context.Context, agent middleware.Agent, input *middleware.ActingInput, next middleware.ActingNext) (*tool.Response, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_acting_"+input.ToolName)
	defer span.End()
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnModelCall(ctx context.Context, agent middleware.Agent, input *middleware.ModelCallInput, next middleware.ModelCallNext) (*message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_model_call")
	defer span.End()
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnSystemPrompt(ctx context.Context, agent middleware.Agent, currentPrompt string) (string, error) {
	_, span := t.Tracer.Start(ctx, t.Name+"_on_system_prompt")
	defer span.End()
	// For tracing, we can log the prompt but typically don't modify unless needed
	return currentPrompt, nil
}

var _ middleware.ReplyInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.ReasoningInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.ActingInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.ModelCallInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.SystemPromptTransformer = (*TracingMiddlewareAdapter)(nil)

// RecordingTracer is a simple in-memory Tracer for demos and tests.
// It records all started spans so you can inspect what was traced.
// Useful for Phase 5 observability alignment demos.
type RecordingTracer struct {
	Spans []string
}

func (r *RecordingTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	r.Spans = append(r.Spans, name)
	return ctx, noopSpan{}
}

var _ Tracer = (*RecordingTracer)(nil)

// 使用 TracedAgent 进行 agent 级 tracing (推荐简单用法)
func ExampleTracing() {
	// agent := observability.NewTracedAgent("demo", baseAgent).WithTracer(otelTracer)
}

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
