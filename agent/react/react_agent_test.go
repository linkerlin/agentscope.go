package react

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// mockChatModel is a simple model for testing
type mockChatModel struct {
	name  string
	usage model.ChatUsage
}

func (m *mockChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	msg := message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	if m.usage.TotalTokens > 0 {
		msg.Metadata["usage"] = m.usage
	}
	return msg, nil
}

func (m *mockChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "ok"}
	if m.usage.TotalTokens > 0 {
		ch <- &model.StreamChunk{Done: true, Usage: &m.usage}
	} else {
		ch <- &model.StreamChunk{Done: true}
	}
	close(ch)
	return ch, nil
}

func (m *mockChatModel) ModelName() string { return m.name }

func TestReActAgent_Shutdown(t *testing.T) {
	agent, err := Builder().
		Name("Test").
		Model(&mockChatModel{name: "mock"}).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if agent.IsClosed() {
		t.Fatal("expected agent not closed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := agent.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if !agent.IsClosed() {
		t.Fatal("expected agent closed after shutdown")
	}

	_, err = agent.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != ErrAgentClosed {
		t.Fatalf("expected ErrAgentClosed, got: %v", err)
	}
}

func TestReActAgent_Shutdown_WaitForOngoingCall(t *testing.T) {
	agent, err := Builder().
		Name("Test").
		Model(&mockChatModel{name: "mock"}).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = agent.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	}()

	// Give the goroutine time to enter Call
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := agent.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not wait for ongoing call")
	}
}

func TestReActAgent_TotalUsage(t *testing.T) {
	m := &mockChatModel{name: "mock", usage: model.ChatUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}}
	agent, err := Builder().
		Name("Test").
		Model(m).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := agent.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
		if err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}

	u := agent.TotalUsage()
	if u.TotalTokens != 15 {
		t.Fatalf("expected total tokens 15, got %d", u.TotalTokens)
	}
	if u.PromptTokens != 9 {
		t.Fatalf("expected prompt tokens 9, got %d", u.PromptTokens)
	}
	if u.CompletionTokens != 6 {
		t.Fatalf("expected completion tokens 6, got %d", u.CompletionTokens)
	}
}

func TestReActAgent_ContextCancellation(t *testing.T) {
	agent, err := Builder().
		Name("Test").
		Model(&mockChatModel{name: "mock"}).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = agent.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
