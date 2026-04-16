package react

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// mockChatModel is a simple model for testing
type mockChatModel struct {
	name         string
	usage        model.ChatUsage
	lastMessages []*message.Msg
}

func (m *mockChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	m.lastMessages = messages
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

// mockReMeMemory wraps InMemoryMemory and tracks PreReasoningPrepare.
type mockReMeMemory struct {
	*memory.InMemoryMemory
	prepareCalled  bool
	prepareHistory []*message.Msg
}

func newMockReMeMemory() *mockReMeMemory {
	return &mockReMeMemory{InMemoryMemory: memory.NewInMemoryMemory()}
}

func (m *mockReMeMemory) PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *memory.CompactSummary, error) {
	m.prepareCalled = true
	m.prepareHistory = append([]*message.Msg(nil), history...)
	out := append([]*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("[compressed]").Build(),
	}, history...)
	return out, &memory.CompactSummary{Raw: "[compressed]"}, nil
}

// preReplyHook injects a marker message during pre_reply.
type preReplyHook struct{}

func (h *preReplyHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if hCtx.Point == hook.HookPreReply {
		modified := append([]*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("[pre_reply]").Build()}, hCtx.Messages...)
		return &hook.HookResult{InjectMessages: modified}, nil
	}
	return nil, nil
}

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


func TestReActAgent_Observe(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	agent, err := Builder().
		Name("Test").
		Model(&mockChatModel{name: "mock"}).
		Memory(mem).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent("observe me").Build()
	if err := agent.Observe(context.Background(), msg); err != nil {
		t.Fatalf("observe failed: %v", err)
	}

	all, err := mem.GetAll()
	if err != nil {
		t.Fatalf("get all failed: %v", err)
	}
	if len(all) != 1 || all[0].GetTextContent() != "observe me" {
		t.Fatalf("expected 1 message with 'observe me', got %v", all)
	}
}

func TestReActAgent_BuildHistory_AutoPreReasoningPrepare(t *testing.T) {
	mockMem := newMockReMeMemory()
	m := &mockChatModel{name: "mock"}
	agent, err := Builder().
		Name("Test").
		Model(m).
		Memory(mockMem).
		SysPrompt("you are helpful").
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	_, err = agent.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	if !mockMem.prepareCalled {
		t.Fatal("expected PreReasoningPrepare to be called")
	}
	if len(m.lastMessages) == 0 {
		t.Fatal("expected model to receive messages")
	}
	if m.lastMessages[0].GetTextContent() != "[compressed]" {
		t.Fatalf("expected first message to be [compressed], got %s", m.lastMessages[0].GetTextContent())
	}
}

func TestReActAgent_PreReplyHook(t *testing.T) {
	m := &mockChatModel{name: "mock"}
	agent, err := Builder().
		Name("Test").
		Model(m).
		Hooks(&preReplyHook{}).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	_, err = agent.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	if len(m.lastMessages) == 0 {
		t.Fatal("expected model to receive messages")
	}
	if m.lastMessages[0].GetTextContent() != "[pre_reply]" {
		t.Fatalf("expected first message to be [pre_reply], got %s", m.lastMessages[0].GetTextContent())
	}
}
