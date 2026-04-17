package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

type mockAgent struct {
	name string
	resp *message.Msg
	err  error
}

func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, errors.New("no stream")
}

func TestTracedAgentCallbacks(t *testing.T) {
	inner := &mockAgent{
		name: "inner",
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
	}
	var calls, results int
	tw := NewTracedAgent("trace", inner)
	tw.OnCall = func(ctx context.Context, name string, msg *message.Msg) { calls++ }
	tw.OnResult = func(ctx context.Context, name string, resp *message.Msg, err error) { results++ }

	if tw.Name() != "inner" {
		t.Fatal(tw.Name())
	}
	r, err := tw.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err != nil || r.GetTextContent() != "ok" {
		t.Fatal(err, r)
	}
	if calls != 1 || results != 1 {
		t.Fatal(calls, results)
	}

	tw2 := NewTracedAgent("only", nil)
	if tw2.Name() != "only" {
		t.Fatal(tw2.Name())
	}
}

func TestTracedAgentCallError(t *testing.T) {
	inner := &mockAgent{name: "e", err: errors.New("fail")}
	tw := NewTracedAgent("t", inner)
	var sawErr bool
	tw.OnResult = func(ctx context.Context, name string, resp *message.Msg, err error) {
		if err != nil {
			sawErr = true
		}
	}
	_, err := tw.Call(context.Background(), message.NewMsg().Role(message.RoleUser).Build())
	if err == nil || !sawErr {
		t.Fatal(err, sawErr)
	}
}


func TestTracedAgentWithTracer(t *testing.T) {
	inner := &mockAgent{name: "inner"}
	tw := NewTracedAgent("t", inner)
	tw.WithTracer(NoopTracer)
	if tw.Tracer == nil {
		t.Fatal("expected tracer set")
	}
	tw.WithTracer(nil) // no-op
	if tw.Tracer == nil {
		t.Fatal("expected tracer unchanged")
	}
}

func TestTracedAgentCallStream(t *testing.T) {
	inner := &mockAgent{name: "inner", err: errors.New("stream fail")}
	tw := NewTracedAgent("t", inner)
	var sawCall bool
	tw.OnCall = func(ctx context.Context, name string, msg *message.Msg) { sawCall = true }
	_, err := tw.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).Build())
	if err == nil {
		t.Fatal("expected error")
	}
	if !sawCall {
		t.Fatal("expected OnCall invoked")
	}
}

type mockStreamAgent struct {
	name string
	ch   <-chan *message.Msg
}

func (m *mockStreamAgent) Name() string { return m.name }
func (m *mockStreamAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, nil
}
func (m *mockStreamAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return m.ch, nil
}

func TestTracedAgentCallStream_Success(t *testing.T) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)

	tw := NewTracedAgent("t", &mockStreamAgent{name: "streamer", ch: ch})
	got, err := tw.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).Build())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected channel")
	}
}

func TestTraceContextIsValid(t *testing.T) {
	if (TraceContext{TraceID: "t", SpanID: "s"}).IsValid() != true {
		t.Fatal("expected valid")
	}
	if (TraceContext{TraceID: "t"}).IsValid() != false {
		t.Fatal("expected invalid")
	}
	if (TraceContext{}).IsValid() != false {
		t.Fatal("expected invalid")
	}
}

func TestNoopSpan(t *testing.T) {
	var s Span = noopSpan{}
	s.End()
	s.RecordError(errors.New("x"))
}

func TestNoopTracer(t *testing.T) {
	ctx := context.Background()
	ctx2, span := NoopTracer.Start(ctx, "name")
	if ctx2 != ctx {
		t.Fatal("expected same context")
	}
	span.End()
}
