package reflection

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

type mockAgent struct {
	name string
	resp string
	err  error
}

func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := m.resp
	if out == "" {
		out = msg.GetTextContent()
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(out).Build(), nil
}

func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, errors.New("not implemented")
}

var _ agent.Agent = (*mockAgent)(nil)

func TestSelfReflectingAgent_Name(t *testing.T) {
	r := NewSelfReflectingAgent("ref", &mockAgent{name: "w"}, &mockAgent{name: "c"}, func(_, _ *message.Msg) bool { return true }, 3)
	if r.Name() != "ref" {
		t.Fatalf("expected name ref, got %s", r.Name())
	}
}

func TestSelfReflectingAgent_Call_AcceptsFirstDraft(t *testing.T) {
	writer := &mockAgent{name: "writer", resp: "draft1"}
	critic := &mockAgent{name: "critic", resp: "looks good"}
	calls := 0
	judge := func(draft, critique *message.Msg) bool {
		calls++
		return true // accept immediately
	}

	r := NewSelfReflectingAgent("ref", writer, critic, judge, 3)
	resp, err := r.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "draft1" {
		t.Fatalf("expected draft1, got %q", resp.GetTextContent())
	}
	if calls != 1 {
		t.Fatalf("expected 1 judge call, got %d", calls)
	}
}

type statefulMockAgent struct {
	mockAgent
	calls int
	resps []string
}

func (m *statefulMockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	m.calls++
	resp := m.resps[0]
	if len(m.resps) > 1 {
		m.resps = m.resps[1:]
	}
	if strings.Contains(msg.GetTextContent(), "feedback") {
		resp = "draft2"
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(resp).Build(), nil
}

func TestSelfReflectingAgent_Call_Revises(t *testing.T) {
	writer := &statefulMockAgent{resps: []string{"draft1"}}
	critic := &mockAgent{name: "critic", resp: "needs more detail"}
	attempts := 0
	judge := func(draft, critique *message.Msg) bool {
		attempts++
		return attempts >= 2 // reject first, accept second revision
	}

	r := NewSelfReflectingAgent("ref", writer, critic, judge, 3)
	resp, err := r.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
	if err != nil {
		t.Fatal(err)
	}
	if writer.calls != 2 {
		t.Fatalf("expected 2 writer calls, got %d", writer.calls)
	}
	if resp.GetTextContent() != "draft2" {
		t.Fatalf("expected draft2, got %q", resp.GetTextContent())
	}
}

func TestSelfReflectingAgent_Call_MaxIter(t *testing.T) {
	writer := &mockAgent{name: "writer", resp: "draft"}
	critic := &mockAgent{name: "critic", resp: "bad"}
	iter := 0
	judge := func(_, _ *message.Msg) bool {
		iter++
		return false // never accept
	}

	r := NewSelfReflectingAgent("ref", writer, critic, judge, 2)
	resp, err := r.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
	if err != nil {
		t.Fatal(err)
	}
	if iter != 2 {
		t.Fatalf("expected 2 iterations, got %d", iter)
	}
	if resp.GetTextContent() != "draft" {
		t.Fatalf("expected final draft, got %q", resp.GetTextContent())
	}
}

func TestSelfReflectingAgent_Call_WriterError(t *testing.T) {
	writer := &mockAgent{name: "writer", err: errors.New("fail")}
	critic := &mockAgent{name: "critic", resp: "ok"}
	r := NewSelfReflectingAgent("ref", writer, critic, func(_, _ *message.Msg) bool { return true }, 3)
	_, err := r.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelfReflectingAgent_Call_CriticError(t *testing.T) {
	writer := &mockAgent{name: "writer", resp: "draft"}
	critic := &mockAgent{name: "critic", err: errors.New("fail")}
	r := NewSelfReflectingAgent("ref", writer, critic, func(_, _ *message.Msg) bool { return true }, 3)
	_, err := r.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelfReflectingAgent_Call_NilDeps(t *testing.T) {
	r := NewSelfReflectingAgent("ref", nil, nil, nil, 3)
	_, err := r.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("topic").Build())
	if err == nil {
		t.Fatal("expected error for nil deps")
	}
}

func TestSelfReflectingAgent_CallStream_NotSupported(t *testing.T) {
	r := NewSelfReflectingAgent("ref", &mockAgent{name: "w"}, &mockAgent{name: "c"}, func(_, _ *message.Msg) bool { return true }, 3)
	_, err := r.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}
