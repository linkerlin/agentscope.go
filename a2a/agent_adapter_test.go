package a2a

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

type mockAgent struct {
	resp   *message.Msg
	err    error
	stream []*message.Msg
}

func (m *mockAgent) Name() string { return "mock" }

func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build(), nil
}

func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *message.Msg, len(m.stream))
	for _, s := range m.stream {
		ch <- s
	}
	close(ch)
	return ch, nil
}

func TestAgentAdapter_Run(t *testing.T) {
	a := &mockAgent{
		resp: message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(),
	}
	adapter := NewAgentAdapter(a)

	msg := &Message{Role: "user", Content: "hi"}
	resp, err := adapter.Run(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Fatalf("expected 'hello', got %q", resp.Content)
	}
	if resp.Role != "agent" {
		t.Fatalf("expected role 'agent', got %q", resp.Role)
	}
}

func TestAgentAdapter_Run_Error(t *testing.T) {
	a := &mockAgent{err: errors.New("fail")}
	adapter := NewAgentAdapter(a)

	_, err := adapter.Run(context.Background(), &Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAgentAdapter_RunStream(t *testing.T) {
	a := &mockAgent{
		stream: []*message.Msg{
			message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build(),
			message.NewMsg().Role(message.RoleAssistant).TextContent(" world").Build(),
		},
	}
	adapter := NewAgentAdapter(a)

	ch, err := adapter.RunStream(context.Background(), &Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}

	var contents []string
	for msg := range ch {
		if msg != nil {
			contents = append(contents, msg.Content)
		}
	}
	if len(contents) != 2 || contents[0] != "hello" || contents[1] != " world" {
		t.Fatalf("unexpected stream contents: %v", contents)
	}
}

func TestAgentAdapter_RunStream_Error(t *testing.T) {
	a := &mockAgent{err: errors.New("stream fail")}
	adapter := NewAgentAdapter(a)

	_, err := adapter.RunStream(context.Background(), &Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAgentAdapter_toMsg(t *testing.T) {
	m := toMsg(&Message{Role: "user", Content: "hello"})
	if m.Role != message.RoleUser {
		t.Fatalf("expected role user, got %s", m.Role)
	}
	if m.GetTextContent() != "hello" {
		t.Fatalf("expected 'hello', got %q", m.GetTextContent())
	}
}

func TestAgentAdapter_fromMsg(t *testing.T) {
	m := message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build()
	a2aMsg := fromMsg(m)
	if a2aMsg.Role != "agent" {
		t.Fatalf("expected role agent, got %s", a2aMsg.Role)
	}
	if a2aMsg.Content != "hello" {
		t.Fatalf("expected 'hello', got %q", a2aMsg.Content)
	}

	// Test other roles pass through
	m2 := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	a2aMsg2 := fromMsg(m2)
	if a2aMsg2.Role != "user" {
		t.Fatalf("expected role user, got %s", a2aMsg2.Role)
	}
}
