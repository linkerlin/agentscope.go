package a2a

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// mockV2AgentForA2A is a minimal V2Agent for adapter testing.
type mockV2AgentForA2A struct{}

func (m *mockV2AgentForA2A) Name() string { return "mock-v2" }
func (m *mockV2AgentForA2A) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}
func (m *mockV2AgentForA2A) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}
func (m *mockV2AgentForA2A) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}
func (m *mockV2AgentForA2A) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 3)
	ch <- event.NewReplyStart("r1", "mock-v2")
	ch <- event.NewTextBlockDelta("r1", 0, "hello")
	ch <- event.NewReplyEnd("r1", "mock-v2")
	close(ch)
	return ch, nil
}
func (m *mockV2AgentForA2A) LoadState(state *agent.AgentState) error { return nil }
func (m *mockV2AgentForA2A) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *mockV2AgentForA2A) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func TestV2AgentAdapter_Run(t *testing.T) {
	adapter := NewV2AgentAdapter(&mockV2AgentForA2A{})
	ctx := context.Background()
	msg := &Message{Role: "user", Content: "hi"}

	resp, err := adapter.Run(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Role != "agent" {
		t.Fatalf("expected role agent, got %s", resp.Role)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected content 'ok', got %s", resp.Content)
	}
}

func TestV2AgentAdapter_RunStream(t *testing.T) {
	adapter := NewV2AgentAdapter(&mockV2AgentForA2A{})
	ctx := context.Background()
	msg := &Message{Role: "user", Content: "hi"}

	ch, err := adapter.RunStream(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}

	var events []string
	for m := range ch {
		if m == nil {
			continue
		}
		et, _ := m.Meta["event_type"].(string)
		events = append(events, et)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[0] != "text_block_delta" {
		t.Fatalf("expected first event text_block_delta, got %s", events[0])
	}
	if events[len(events)-1] != "reply_end" {
		t.Fatalf("expected last event reply_end, got %s", events[len(events)-1])
	}
}

func TestV2AgentAdapter_InterfaceCompliance(t *testing.T) {
	var _ AgentRunner = (*V2AgentAdapter)(nil)
	var _ StreamingAgentRunner = (*V2AgentAdapter)(nil)
}

func TestV2AgentAdapter_RunStream_Error(t *testing.T) {
	// Use an agent that returns error
	errAgent := &mockV2AgentErr{}
	adapter := NewV2AgentAdapter(errAgent)
	ctx := context.Background()
	msg := &Message{Role: "user", Content: "hi"}

	_, err := adapter.RunStream(ctx, msg)
	if err == nil {
		t.Fatal("expected error from failing agent")
	}
}

type mockV2AgentErr struct{}

func (m *mockV2AgentErr) Name() string { return "err" }
func (m *mockV2AgentErr) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, context.Canceled
}
func (m *mockV2AgentErr) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, context.Canceled
}
func (m *mockV2AgentErr) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, context.Canceled
}
func (m *mockV2AgentErr) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	return nil, context.Canceled
}
func (m *mockV2AgentErr) LoadState(state *agent.AgentState) error { return nil }
func (m *mockV2AgentErr) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *mockV2AgentErr) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}
