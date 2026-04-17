package workflow

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

type mockAgent struct {
	mu        sync.Mutex
	name      string
	resp      string
	err       error
	lastInput string
}

func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	m.mu.Lock()
	m.lastInput = msg.GetTextContent()
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	out := m.resp
	if out == "" {
		out = msg.GetTextContent()
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(out).Build(), nil
}

func (m *mockAgent) getLastInput() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastInput
}

func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, errors.New("not implemented")
}

var _ agent.Agent = (*mockAgent)(nil)

// ---- Parallel ----

func TestParallel_Name(t *testing.T) {
	p := NewParallel("p", nil)
	if p.Name() != "p" {
		t.Fatalf("expected name p, got %s", p.Name())
	}
}

func TestParallel_Call_Success(t *testing.T) {
	a1 := &mockAgent{name: "a1", resp: "hello"}
	a2 := &mockAgent{name: "a2", resp: "world"}
	p := NewParallel("p", nil, a1, a2)

	resp, err := p.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	text := resp.GetTextContent()
	if !strings.Contains(text, "hello") || !strings.Contains(text, "world") {
		t.Fatalf("expected merged text containing 'hello' and 'world', got %q", text)
	}
}

func TestParallel_Call_EmptyItems(t *testing.T) {
	p := NewParallel("p", nil)
	_, err := p.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error for empty parallel items")
	}
}

func TestParallel_Call_ErrorItem(t *testing.T) {
	a1 := &mockAgent{name: "a1", resp: "ok"}
	a2 := &mockAgent{name: "a2", err: errors.New("boom")}
	p := NewParallel("p", nil, a1, a2)

	resp, err := p.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "ok") {
		t.Fatalf("expected 'ok' in output, got %q", resp.GetTextContent())
	}
	if !strings.Contains(resp.GetTextContent(), "boom") {
		t.Fatalf("expected error message in output, got %q", resp.GetTextContent())
	}
}

func TestParallel_CallStream_NotSupported(t *testing.T) {
	p := NewParallel("p", nil, &mockAgent{name: "a"})
	_, err := p.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- Condition ----

func TestCondition_Call_TrueBranch(t *testing.T) {
	trueAgent := &mockAgent{name: "trueAgent", resp: "yes"}
	falseAgent := &mockAgent{name: "falseAgent", resp: "no"}
	c := NewCondition("c", func(m *message.Msg) bool { return strings.Contains(m.GetTextContent(), "go") }, trueAgent, falseAgent)

	resp, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("go ahead").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "yes" {
		t.Fatalf("expected 'yes', got %q", resp.GetTextContent())
	}
}

func TestCondition_Call_FalseBranch(t *testing.T) {
	trueAgent := &mockAgent{name: "trueAgent", resp: "yes"}
	falseAgent := &mockAgent{name: "falseAgent", resp: "no"}
	c := NewCondition("c", func(m *message.Msg) bool { return strings.Contains(m.GetTextContent(), "go") }, trueAgent, falseAgent)

	resp, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("stop").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "no" {
		t.Fatalf("expected 'no', got %q", resp.GetTextContent())
	}
}

func TestCondition_Call_NilEvaluator(t *testing.T) {
	c := NewCondition("c", nil, &mockAgent{name: "t"}, &mockAgent{name: "f"})
	_, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err == nil {
		t.Fatal("expected error for nil evaluator")
	}
}

func TestCondition_Call_NilBranch(t *testing.T) {
	c := NewCondition("c", func(m *message.Msg) bool { return true }, nil, nil)
	_, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("x").Build())
	if err == nil {
		t.Fatal("expected error for nil branch")
	}
}

// ---- Loop ----

func TestLoop_Call_Success(t *testing.T) {
	counter := 0
	body := &mockAgent{name: "body", resp: "step"}
	// condition: continue while counter < 3
	c := NewLoop("loop", body, func(m *message.Msg) bool {
		counter++
		return counter < 3
	}, 10)

	resp, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "step" {
		t.Fatalf("expected 'step', got %q", resp.GetTextContent())
	}
	if counter != 3 {
		t.Fatalf("expected 3 iterations, got %d", counter)
	}
}

func TestLoop_Call_MaxIter(t *testing.T) {
	iter := 0
	body := &mockAgent{name: "body", resp: "x"}
	c := NewLoop("loop", body, func(m *message.Msg) bool {
		iter++
		return true // never stops naturally
	}, 5)

	_, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err != nil {
		t.Fatal(err)
	}
	if iter != 5 {
		t.Fatalf("expected max 5 iterations, got %d", iter)
	}
}

func TestLoop_Call_BodyError(t *testing.T) {
	body := &mockAgent{name: "body", err: errors.New("fail")}
	c := NewLoop("loop", body, func(m *message.Msg) bool { return true }, 10)
	_, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoop_Call_NilBody(t *testing.T) {
	c := NewLoop("loop", nil, func(m *message.Msg) bool { return false }, 10)
	_, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err == nil {
		t.Fatal("expected error for nil body")
	}
}

func TestLoop_Call_NilCondition(t *testing.T) {
	c := NewLoop("loop", &mockAgent{name: "body"}, nil, 10)
	_, err := c.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("start").Build())
	if err == nil {
		t.Fatal("expected error for nil condition")
	}
}
