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
