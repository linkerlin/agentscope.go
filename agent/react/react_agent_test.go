package react

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/interruption"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/shutdown"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit"
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

// mockToolModel returns preset responses sequentially.
type mockToolModel struct {
	responses []*message.Msg
	calls     int
	name      string
	err       error
}

func (m *mockToolModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	resp := m.responses[m.calls%len(m.responses)]
	m.calls++
	return resp, nil
}

func (m *mockToolModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "stream"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockToolModel) ModelName() string { return m.name }

// mockTCRMemory supports AddToolCallResult and SummarizeToolUsage.
type mockTCRMemory struct {
	*memory.InMemoryMemory
	added      []memory.ToolCallResult
	summarized []string
}

func (m *mockTCRMemory) AddToolCallResult(ctx context.Context, result memory.ToolCallResult) error {
	m.added = append(m.added, result)
	return nil
}

func (m *mockTCRMemory) SummarizeToolUsage(ctx context.Context, toolName string) error {
	m.summarized = append(m.summarized, toolName)
	return nil
}

// mockPromptMemory supports GetMemoryForPrompt.
type mockPromptMemory struct {
	*memory.InMemoryMemory
	promptMsgs []*message.Msg
}

func (m *mockPromptMemory) GetMemoryForPrompt(prepend bool) ([]*message.Msg, error) {
	return append([]*message.Msg(nil), m.promptMsgs...), nil
}

// badMemory always errors on GetAll.
type badMemory struct{}

func (b *badMemory) GetAll() ([]*message.Msg, error)   { return nil, errors.New("bad memory") }
func (b *badMemory) Add(msg *message.Msg) error        { return nil }
func (b *badMemory) GetRecent(n int) ([]*message.Msg, error) { return nil, nil }
func (b *badMemory) Clear() error                      { return nil }
func (b *badMemory) Size() int                         { return 0 }

func TestReActAgent_BuilderChainAndDefaults(t *testing.T) {
	tk := toolkit.NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("echo", "echo", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("echo"), nil
	}))
	hm := hook.NewManager()
	hm.Register(hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) { return nil, nil }))

	a, err := Builder().
		ID("aid").
		Name("n").
		Description("d").
		Metadata(map[string]any{"k": "v"}).
		SysPrompt("sys").
		Model(&mockChatModel{name: "m"}).
		Tools(tool.NewFunctionTool("t1", "t1", nil, nil)).
		Toolkit(tk).
		MaxIterations(5).
		Hooks(hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) { return nil, nil })).
		HookManager(hm).
		StreamHooks(hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) { return nil, nil })).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if a.Name() != "n" {
		t.Fatalf("expected name n, got %s", a.Name())
	}
	if _, ok := a.toolMap["echo"]; !ok {
		t.Fatal("expected toolkit tool echo in toolMap")
	}
	if _, ok := a.toolMap["t1"]; ok {
		t.Fatal("expected tools to be overridden by toolkit")
	}
	if a.maxIterations != 5 {
		t.Fatalf("expected maxIterations 5, got %d", a.maxIterations)
	}
	if a.Base.ID != "aid" {
		t.Fatalf("expected ID aid, got %s", a.Base.ID)
	}
}

func TestReActAgent_CallStream(t *testing.T) {
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Build()
	ch, err := a.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	var got []*message.Msg
	for m := range ch {
		got = append(got, m)
	}
	if len(got) != 1 || got[0].GetTextContent() != "ok" {
		t.Fatalf("unexpected stream result: %v", got)
	}

	a2, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Build()
	_ = a2.Shutdown(context.Background())
	ch2, _ := a2.CallStream(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	var got2 []*message.Msg
	for m := range ch2 {
		got2 = append(got2, m)
	}
	if len(got2) != 1 || got2[0].GetTextContent() != "error: react agent: agent is closed" {
		t.Fatalf("unexpected error stream result: %v", got2)
	}
}

func TestReActAgent_ToolExecutionAndSummarize(t *testing.T) {
	echoTool := tool.NewFunctionTool("echo", "echo", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("done"), nil
	})
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{"x": 1}),
	).Build()
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("final").Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg, finalMsg}}

	mockMem := &mockTCRMemory{InMemoryMemory: memory.NewInMemoryMemory()}
	a, err := Builder().Name("Test").Model(m).Tools(echoTool).Memory(mockMem).Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "final" {
		t.Fatalf("expected final, got %s", resp.GetTextContent())
	}
	if len(mockMem.added) != 1 {
		t.Fatalf("expected 1 tool call result, got %d", len(mockMem.added))
	}
	if len(mockMem.summarized) != 1 || mockMem.summarized[0] != "echo" {
		t.Fatalf("expected summarize echo, got %v", mockMem.summarized)
	}
}

func TestReActAgent_ToolExecutionViaToolkit(t *testing.T) {
	tk := toolkit.NewToolkit()
	_ = tk.Register(tool.NewFunctionTool("kit_echo", "kit_echo", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("kit_done"), nil
	}))
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "kit_echo", map[string]any{}),
	).Build()
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("final").Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg, finalMsg}}

	a, err := Builder().Name("Test").Model(m).Toolkit(tk).Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "final" {
		t.Fatalf("expected final, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_ToolNotFound(t *testing.T) {
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "missing", map[string]any{}),
	).Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg}}
	a, _ := Builder().Name("Test").Model(m).Build()
	_, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestReActAgent_MaxIterationsReached(t *testing.T) {
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{}),
	).Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg, toolCallMsg, toolCallMsg}}
	echo := tool.NewFunctionTool("echo", "echo", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("ok"), nil
	})
	a, _ := Builder().Name("Test").Model(m).Tools(echo).MaxIterations(2).Build()
	_, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil || err.Error() != "react agent: max iterations reached without final answer" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReActAgent_HookBeforeModel_Interrupt(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookBeforeModel {
			return &hook.HookResult{Interrupt: true, Override: message.NewMsg().Role(message.RoleAssistant).TextContent("intr").Build()}, nil
		}
		return nil, nil
	})
	m := &mockChatModel{name: "m"}
	a, _ := Builder().Name("Test").Model(m).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "intr" {
		t.Fatalf("expected intr, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookBeforeModel_Override(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookBeforeModel {
			return &hook.HookResult{Override: message.NewMsg().Role(message.RoleAssistant).TextContent("ovr").Build()}, nil
		}
		return nil, nil
	})
	m := &mockChatModel{name: "m"}
	a, _ := Builder().Name("Test").Model(m).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ovr" {
		t.Fatalf("expected ovr, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookAfterModel_StopAgent(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookAfterModel {
			return &hook.HookResult{StopAgent: true, Override: message.NewMsg().Role(message.RoleAssistant).TextContent("stop").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "stop" {
		t.Fatalf("expected stop, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookAfterModel_GotoReasoning(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookAfterModel && len(hCtx.Messages) == 1 {
			return &hook.HookResult{GotoReasoning: true, GotoReasoningMsgs: []*message.Msg{
				message.NewMsg().Role(message.RoleUser).TextContent("[goto]").Build(),
			}}, nil
		}
		return nil, nil
	})
	m := &mockToolModel{name: "m", responses: []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
	}}
	a, _ := Builder().Name("Test").Model(m).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("expected ok, got %s", resp.GetTextContent())
	}
	if m.calls != 2 {
		t.Fatalf("expected 2 model calls, got %d", m.calls)
	}
}

func TestReActAgent_HookAfterModel_Interrupt(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookAfterModel {
			return &hook.HookResult{Interrupt: true, Override: message.NewMsg().Role(message.RoleAssistant).TextContent("intr2").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "intr2" {
		t.Fatalf("expected intr2, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookAfterModel_Override(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookAfterModel {
			return &hook.HookResult{Override: message.NewMsg().Role(message.RoleAssistant).TextContent("ovr2").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ovr2" {
		t.Fatalf("expected ovr2, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookBeforeFinish_Override(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookBeforeFinish {
			return &hook.HookResult{Override: message.NewMsg().Role(message.RoleAssistant).TextContent("fin").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "fin" {
		t.Fatalf("expected fin, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookBeforeTool_Interrupt(t *testing.T) {
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{}),
	).Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg}}
	echo := tool.NewFunctionTool("echo", "echo", nil, nil)
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookBeforeTool {
			return &hook.HookResult{Interrupt: true, Override: message.NewMsg().Role(message.RoleAssistant).TextContent("tintr").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(m).Tools(echo).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "tintr" {
		t.Fatalf("expected tintr, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookAfterTool_StopAgent(t *testing.T) {
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{}),
	).Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg}}
	echo := tool.NewFunctionTool("echo", "echo", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("res"), nil
	})
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookAfterTool {
			return &hook.HookResult{StopAgent: true, Override: message.NewMsg().Role(message.RoleAssistant).TextContent("tstop").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(m).Tools(echo).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "tstop" {
		t.Fatalf("expected tstop, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_HookAfterTool_Interrupt(t *testing.T) {
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{}),
	).Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg}}
	echo := tool.NewFunctionTool("echo", "echo", nil, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("res"), nil
	})
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookAfterTool {
			return &hook.HookResult{Interrupt: true, Override: message.NewMsg().Role(message.RoleAssistant).TextContent("tintr2").Build()}, nil
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(m).Tools(echo).Hooks(h).Build()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "tintr2" {
		t.Fatalf("expected tintr2, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_BuildHistory_GetMemoryForPrompt(t *testing.T) {
	pm := &mockPromptMemory{InMemoryMemory: memory.NewInMemoryMemory(), promptMsgs: []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("pm").Build(),
	}}
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Memory(pm).Build()
	hist, err := a.buildHistory(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 || hist[0].GetTextContent() != "pm" {
		t.Fatalf("unexpected history: %v", hist)
	}
}

func TestReActAgent_ReplyInternal_BuildHistoryError(t *testing.T) {
	badMem := &badMemory{}
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).Memory(badMem).Build()
	_, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReActAgent_ReplyInternal_StreamEventError(t *testing.T) {
	sh := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		if ev.EventType() == hook.EventPreCall {
			return nil, errors.New("stream err")
		}
		return nil, nil
	})
	a, _ := Builder().Name("Test").Model(&mockChatModel{name: "m"}).StreamHooks(sh).Build()
	_, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil || err.Error() != "stream err" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractUsage_NonChatUsage(t *testing.T) {
	msg := message.NewMsg().Role(message.RoleAssistant).TextContent("x").Metadata("usage", "not usage").Build()
	u := extractUsage(msg)
	if u.TotalTokens != 0 {
		t.Fatalf("expected zero usage, got %+v", u)
	}
}

func TestReActAgent_UserInterrupt(t *testing.T) {
	// Slow model so the goroutine has time to fire the interrupt.
	m := &slowMockChatModel{name: "mock", delay: 100 * time.Millisecond}
	a, err := Builder().
		Name("Test").
		Model(m).
		MaxIterations(3).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		a.Interrupt()
	}()

	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "I noticed that you have interrupted me. What can I do for you?" {
		t.Fatalf("unexpected recovery text: %s", resp.GetTextContent())
	}
}

func TestReActAgent_SystemInterrupt_Save(t *testing.T) {
	m := &slowMockChatModel{name: "mock", delay: 100 * time.Millisecond}
	mem := memory.NewInMemoryMemory()
	a, err := Builder().
		Name("Test").
		Model(m).
		Memory(mem).
		MaxIterations(3).
		ShutdownConfig(shutdown.GracefulShutdownConfig{
			PartialReasoningPolicy: shutdown.Save,
		}).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		a.InterruptWithSource(interruption.SourceSystem)
	}()

	_, err = a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected shutdown error")
	}
	if !errors.Is(err, ErrAgentClosed) {
		t.Fatalf("expected ErrAgentClosed wrapped error, got %v", err)
	}
}

func TestReActAgent_SystemInterrupt_Discard(t *testing.T) {
	m := &slowMockChatModel{name: "mock", delay: 100 * time.Millisecond}
	mem := memory.NewInMemoryMemory()
	a, err := Builder().
		Name("Test").
		Model(m).
		Memory(mem).
		MaxIterations(3).
		ShutdownConfig(shutdown.GracefulShutdownConfig{
			PartialReasoningPolicy: shutdown.Discard,
		}).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		a.InterruptWithSource(interruption.SourceSystem)
	}()

	_, err = a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err == nil {
		t.Fatal("expected shutdown error")
	}
}

func TestReActAgent_InterruptDuringToolExecution(t *testing.T) {
	// Slow tool-call model so the interrupt goroutine wins the race.
	toolModel := &slowToolCallModel{name: "tc", toolName: "dummy", delay: 100 * time.Millisecond}
	dummy := tool.NewFunctionTool("dummy", "", map[string]any{}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("done"), nil
	})

	a, err := Builder().
		Name("Test").
		Model(toolModel).
		Tools(dummy).
		MaxIterations(3).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		a.Interrupt()
	}()

	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "I noticed that you have interrupted me. What can I do for you?" {
		t.Fatalf("unexpected recovery text: %s", resp.GetTextContent())
	}
}

// slowMockChatModel sleeps before returning a text response.
type slowMockChatModel struct {
	name  string
	delay time.Duration
}

func (m *slowMockChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	time.Sleep(m.delay)
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *slowMockChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, errors.New("not supported")
}

func (m *slowMockChatModel) ModelName() string { return m.name }

// slowToolCallModel sleeps before returning a tool_use block.
type slowToolCallModel struct {
	name     string
	toolName string
	delay    time.Duration
}

func (m *slowToolCallModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	time.Sleep(m.delay)
	msg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("tc-1", m.toolName, map[string]any{}),
	).Build()
	return msg, nil
}

func (m *slowToolCallModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, errors.New("not supported")
}

func (m *slowToolCallModel) ModelName() string { return m.name }


// trackingHook records which points were fired.
type trackingHook struct {
	points []hook.HookPoint
}

func (h *trackingHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	h.points = append(h.points, hCtx.Point)
	return nil, nil
}

func TestReActAgent_HookPreCall(t *testing.T) {
	h := &trackingHook{}
	a, err := Builder().
		Name("Test").
		Model(&mockChatModel{name: "mock"}).
		Hooks(h).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	var hasPreCall bool
	for _, p := range h.points {
		if p == hook.HookPreCall {
			hasPreCall = true
			break
		}
	}
	if !hasPreCall {
		t.Fatalf("expected HookPreCall to be fired, got %v", h.points)
	}
}

func TestReActAgent_HookPreCall_Override(t *testing.T) {
	h := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
		if hCtx.Point == hook.HookPreCall {
			return &hook.HookResult{Override: message.NewMsg().Role(message.RoleAssistant).TextContent("pre_call_override").Build()}, nil
		}
		return nil, nil
	})
	a, err := Builder().
		Name("Test").
		Model(&mockChatModel{name: "mock"}).
		Hooks(h).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "pre_call_override" {
		t.Fatalf("expected pre_call_override, got %s", resp.GetTextContent())
	}
}

func TestReActAgent_ConcurrentToolExecution(t *testing.T) {
	var execOrderMu sync.Mutex
	var execOrder []string

	sleepTool := tool.NewFunctionTool("sleep", "sleep", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		ms, _ := input["ms"].(float64)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		execOrderMu.Lock()
		execOrder = append(execOrder, fmt.Sprintf("sleep_%v", ms))
		execOrderMu.Unlock()

		return tool.NewTextResponse(fmt.Sprintf("slept %v", ms)), nil
	})

	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "sleep", map[string]any{"ms": 100}),
		message.NewToolUseBlock("call_2", "sleep", map[string]any{"ms": 100}),
	).Build()
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("final").Build()
	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg, finalMsg}}

	a, err := Builder().Name("Test").Model(m).Tools(sleepTool).Build()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if resp.GetTextContent() != "final" {
		t.Fatalf("expected final, got %s", resp.GetTextContent())
	}

	// Concurrent execution means total time should be less than sequential sum (200ms).
	if elapsed >= 180*time.Millisecond {
		t.Fatalf("expected concurrent execution (< 180ms), took %v", elapsed)
	}

	execOrderMu.Lock()
	if len(execOrder) != 2 {
		t.Fatalf("expected 2 tool executions, got %d", len(execOrder))
	}
	execOrderMu.Unlock()
}
