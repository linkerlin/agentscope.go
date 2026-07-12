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
	span.SetAttributes(
		IntAttr("reply.message_count", len(input.Messages)),
		StringAttr("reply.last_role", lastRole(input.Messages)),
	)
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnReasoning(ctx context.Context, agent middleware.Agent, input *middleware.ReasoningInput, next middleware.ReasoningNext) (*message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_reasoning")
	defer span.End()
	span.SetAttributes(
		IntAttr("reasoning.iteration", input.Iteration),
		IntAttr("reasoning.message_count", len(input.Messages)),
	)
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnActing(ctx context.Context, agent middleware.Agent, input *middleware.ActingInput, next middleware.ActingNext) (*tool.Response, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_acting")
	defer span.End()
	span.SetAttributes(
		StringAttr("tool.name", input.ToolName),
		IntAttr("tool.input_keys", len(input.ToolInput)),
	)
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnModelCall(ctx context.Context, agent middleware.Agent, input *middleware.ModelCallInput, next middleware.ModelCallNext) (*message.Msg, error) {
	ctx, span := t.Tracer.Start(ctx, t.Name+"_on_model_call")
	defer span.End()
	span.SetAttributes(
		StringAttr("model.name", input.ModelName),
		IntAttr("model.message_count", len(input.Messages)),
		IntAttr("model.chat_opts", len(input.ChatOpts)),
	)
	return next(ctx)
}

func (t *TracingMiddlewareAdapter) OnSystemPrompt(ctx context.Context, agent middleware.Agent, currentPrompt string) (string, error) {
	_, span := t.Tracer.Start(ctx, t.Name+"_on_system_prompt")
	defer span.End()
	span.SetAttributes(IntAttr("system_prompt.length", len(currentPrompt)))
	return currentPrompt, nil
}

func lastRole(msgs []*message.Msg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil {
			return string(msgs[i].Role)
		}
	}
	return ""
}

var _ middleware.ReplyInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.ReasoningInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.ActingInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.ModelCallInterceptor = (*TracingMiddlewareAdapter)(nil)
var _ middleware.SystemPromptTransformer = (*TracingMiddlewareAdapter)(nil)

// RecordingSpan captures its name and attributes for later inspection in
// tests/demos. Implements Span.
type RecordingSpan struct {
	Name  string
	Attrs []SpanAttr
	Err   error
	Ended bool
}

func (s *RecordingSpan) End()                            { s.Ended = true }
func (s *RecordingSpan) RecordError(err error)           { s.Err = err }
func (s *RecordingSpan) SetAttributes(attrs ...SpanAttr) { s.Attrs = append(s.Attrs, attrs...) }

func (s *RecordingSpan) Attr(key string) any {
	for _, a := range s.Attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return nil
}

// RecordingTracer is a simple in-memory Tracer for demos and tests. It records
// all started spans (as RecordingSpan) so you can inspect names + attributes.
type RecordingTracer struct {
	Spans []*RecordingSpan
}

func (r *RecordingTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	s := &RecordingSpan{Name: name}
	r.Spans = append(r.Spans, s)
	return ctx, s
}

// LastSpan returns the most recently started span, or nil.
func (r *RecordingTracer) LastSpan() *RecordingSpan {
	if len(r.Spans) == 0 {
		return nil
	}
	return r.Spans[len(r.Spans)-1]
}

// SpanByName returns the first span with the given name, or nil.
func (r *RecordingTracer) SpanByName(name string) *RecordingSpan {
	for _, s := range r.Spans {
		if s.Name == name {
			return s
		}
	}
	return nil
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
