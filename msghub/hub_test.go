package msghub

import (
	"context"
	"errors"
	"sort"
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

func TestHub_RegisterAndGet(t *testing.T) {
	h := New()
	a := &mockAgent{name: "a1"}
	h.Register("a1", a)

	got, ok := h.Get("a1")
	if !ok {
		t.Fatal("expected agent a1 to be found")
	}
	if got.Name() != "a1" {
		t.Fatalf("expected name a1, got %s", got.Name())
	}
}

func TestHub_Unregister(t *testing.T) {
	h := New()
	h.Register("a1", &mockAgent{name: "a1"})
	h.Unregister("a1")

	_, ok := h.Get("a1")
	if ok {
		t.Fatal("expected agent a1 to be unregistered")
	}
}

func TestHub_Names(t *testing.T) {
	h := New()
	h.Register("b", &mockAgent{name: "b"})
	h.Register("a", &mockAgent{name: "a"})

	names := h.Names()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestHub_Send(t *testing.T) {
	h := New()
	h.Register("writer", &mockAgent{name: "writer", resp: "hello"})

	resp, err := h.Send(context.Background(), "writer", message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "hello" {
		t.Fatalf("expected 'hello', got %q", resp.GetTextContent())
	}
}

func TestHub_Send_NotFound(t *testing.T) {
	h := New()
	_, err := h.Send(context.Background(), "missing", message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestHub_Broadcast(t *testing.T) {
	h := New()
	h.Register("a", &mockAgent{name: "a", resp: "A"})
	h.Register("b", &mockAgent{name: "b", resp: "B"})

	results := h.Broadcast(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if len(results) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(results))
	}
	if results["a"].GetTextContent() != "A" {
		t.Fatalf("expected A, got %q", results["a"].GetTextContent())
	}
	if results["b"].GetTextContent() != "B" {
		t.Fatalf("expected B, got %q", results["b"].GetTextContent())
	}
}

func TestHub_Broadcast_Error(t *testing.T) {
	h := New()
	h.Register("ok", &mockAgent{name: "ok", resp: "fine"})
	h.Register("bad", &mockAgent{name: "bad", err: errors.New("boom")})

	results := h.Broadcast(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if results["ok"].GetTextContent() != "fine" {
		t.Fatalf("expected fine, got %q", results["ok"].GetTextContent())
	}
	if results["bad"].GetTextContent() != "error: boom" {
		t.Fatalf("expected error message, got %q", results["bad"].GetTextContent())
	}
}
