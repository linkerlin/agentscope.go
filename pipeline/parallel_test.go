package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
)

// mockParallelAgent is a test double that returns a fixed message.
type mockParallelAgent struct {
	name     string
	latency  time.Duration
	response string
	err      error
}

func (m *mockParallelAgent) Name() string { return m.name }

func (m *mockParallelAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.latency > 0 {
		select {
		case <-time.After(m.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent(m.response).Build(), nil
}

func (m *mockParallelAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, errors.New("not implemented")
}

var _ agent.Agent = (*mockParallelAgent)(nil)

func TestParallel_Basic(t *testing.T) {
	p := NewParallel("test",
		&mockParallelAgent{name: "a", response: "Result A"},
		&mockParallelAgent{name: "b", response: "Result B"},
	)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	out, err := p.Call(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	txt := out.GetTextContent()
	if !strings.Contains(txt, "Result A") || !strings.Contains(txt, "Result B") {
		t.Fatalf("expected both results, got: %s", txt)
	}
}

func TestParallel_EmptySteps(t *testing.T) {
	p := NewParallel("empty")
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	_, err := p.Call(context.Background(), msg)
	if err == nil || !strings.Contains(err.Error(), "no steps") {
		t.Fatalf("expected 'no steps' error, got: %v", err)
	}
}

func TestParallel_StepError(t *testing.T) {
	p := NewParallel("test",
		&mockParallelAgent{name: "a", response: "Result A"},
		&mockParallelAgent{name: "b", err: errors.New("boom")},
	)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	_, err := p.Call(context.Background(), msg)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected step error, got: %v", err)
	}
}

func TestParallel_Concurrency(t *testing.T) {
	start := time.Now()
	p := NewParallel("test",
		&mockParallelAgent{name: "a", latency: 100 * time.Millisecond, response: "A"},
		&mockParallelAgent{name: "b", latency: 100 * time.Millisecond, response: "B"},
		&mockParallelAgent{name: "c", latency: 100 * time.Millisecond, response: "C"},
	)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	_, err := p.Call(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected concurrent execution (~100ms), took %v", elapsed)
	}
}

func TestParallel_CustomAggregator(t *testing.T) {
	p := NewParallel("test",
		&mockParallelAgent{name: "a", response: "A"},
		&mockParallelAgent{name: "b", response: "B"},
	).WithAggregator(func(msgs []*message.Msg) *message.Msg {
		var sum int
		for _, m := range msgs {
			sum += len(m.GetTextContent())
		}
		return message.NewMsg().Role(message.RoleAssistant).TextContent(
			string(rune('0'+sum)),
		).Build()
	})

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	out, err := p.Call(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A(1) + B(1) = 2
	if out.GetTextContent() != string(rune('0'+2)) {
		t.Fatalf("unexpected aggregated result: %s", out.GetTextContent())
	}
}

func TestParallel_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	p := NewParallel("test",
		&mockParallelAgent{name: "a", latency: 200 * time.Millisecond, response: "A"},
	)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	_, err := p.Call(ctx, msg)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestParallel_CallStream_NotSupported(t *testing.T) {
	p := NewParallel("test", &mockParallelAgent{name: "a", response: "A"})
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	_, err := p.CallStream(context.Background(), msg)
	if err == nil || !strings.Contains(err.Error(), "streaming not supported") {
		t.Fatalf("expected streaming not supported, got: %v", err)
	}
}
