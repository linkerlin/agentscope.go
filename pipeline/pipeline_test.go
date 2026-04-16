package pipeline

import (
	"context"
	"errors"
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

func TestPipeline_Name(t *testing.T) {
	p := New("test-pipe")
	if p.Name() != "test-pipe" {
		t.Fatalf("expected name test-pipe, got %s", p.Name())
	}
}

func TestPipeline_Call_Success(t *testing.T) {
	step1 := &mockAgent{name: "step1", resp: "step1-out"}
	step2 := &mockAgent{name: "step2", resp: "step2-out"}
	p := New("pipe", step1, step2)

	resp, err := p.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hello").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "step2-out" {
		t.Fatalf("expected 'step2-out', got %q", resp.GetTextContent())
	}
	if resp.Role != message.RoleAssistant {
		t.Fatalf("expected assistant role, got %s", resp.Role)
	}
}

func TestPipeline_Call_EmptySteps(t *testing.T) {
	p := New("empty")
	_, err := p.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error for empty pipeline")
	}
}

func TestPipeline_Call_StepError(t *testing.T) {
	step1 := &mockAgent{name: "step1", resp: "ok"}
	step2 := &mockAgent{name: "step2", err: errors.New("boom")}
	p := New("pipe", step1, step2)

	_, err := p.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hello").Build())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, step2.err) {
		t.Fatalf("expected error to wrap step2.err, got %v", err)
	}
}

func TestPipeline_CallStream_NotSupported(t *testing.T) {
	p := New("pipe", &mockAgent{name: "step1"})
	_, err := p.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}
