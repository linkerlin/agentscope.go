package react

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

type mockModel struct {
	name      string
	ch        chan *model.StreamChunk
	chatErr   error
	streamErr error
}

func (m *mockModel) ModelName() string { return m.name }

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("sync").Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return m.ch, nil
}

func TestRunModelStreamChunks(t *testing.T) {
	ch := make(chan *model.StreamChunk, 4)
	ch <- &model.StreamChunk{Delta: "a"}
	ch <- &model.StreamChunk{Delta: "b"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)

	var chunks int
	sh := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		if ev.EventType() == hook.EventReasoningChunk {
			chunks++
		}
		return nil, nil
	})

	a := &ReActAgent{
		Base:          agent.NewBase("", "t", "", "", nil, nil, []hook.StreamHook{sh}),
		chatModel:     &mockModel{name: "m", ch: ch},
		memory:        memory.NewInMemoryMemory(),
		maxIterations: 3,
		toolMap:       map[string]tool.Tool{},
	}
	hist := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}
	msg, err := a.runModel(context.Background(), hist, nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "ab" {
		t.Fatalf("got %q", msg.GetTextContent())
	}
	if chunks != 2 {
		t.Fatalf("chunks=%d", chunks)
	}
}

func TestRunModelChatStreamError(t *testing.T) {
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, []hook.StreamHook{hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) { return nil, nil })}),
		chatModel: &mockModel{name: "m", streamErr: errors.New("stream err")},
		memory:    memory.NewInMemoryMemory(),
	}
	_, err := a.runModel(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}, nil, 0, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunModelChatError(t *testing.T) {
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, nil),
		chatModel: &mockModel{name: "m", chatErr: errors.New("chat err")},
		memory:    memory.NewInMemoryMemory(),
	}
	_, err := a.runModel(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}, nil, 0, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunModelNilChunk(t *testing.T) {
	ch := make(chan *model.StreamChunk, 3)
	ch <- &model.StreamChunk{Delta: "a"}
	ch <- nil
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, []hook.StreamHook{hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) { return nil, nil })}),
		chatModel: &mockModel{name: "m", ch: ch},
		memory:    memory.NewInMemoryMemory(),
	}
	msg, err := a.runModel(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}, nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "a" {
		t.Fatalf("expected a, got %s", msg.GetTextContent())
	}
}

// recordingModel captures the ChatOptions and messages the model actually
// receives, so the framework-fix test can assert middleware mutations propagate.
type recordingModel struct {
	name    string
	gotOpts []model.ChatOption
	gotMsgs []*message.Msg
}

func (m *recordingModel) ModelName() string { return m.name }
func (m *recordingModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	m.gotOpts = opts
	m.gotMsgs = msgs
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}
func (m *recordingModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	return nil, errors.New("not used")
}

// forceNoneReasoningMW is an on_reasoning middleware that mutates ChatOpts
// (force tool_choice=none) and injects a marker message into the input.
type forceNoneReasoningMW struct{ middleware.Base }

func (m *forceNoneReasoningMW) OnReasoning(ctx context.Context, ag middleware.Agent, input *middleware.ReasoningInput, next middleware.ReasoningNext) (*message.Msg, error) {
	input.ChatOpts = append(input.ChatOpts, model.WithToolChoice(&model.ToolChoice{Mode: "none"}))
	input.Messages = append(input.Messages, message.NewMsg().Role(message.RoleAssistant).Name("inject").TextContent("INJECTED-MARKER").Build())
	return next(ctx)
}

// TestRunModel_ReasoningMiddlewareMutationPropagates verifies the Tier 1B
// framework fix: an on_reasoning middleware's mutations to input.ChatOpts and
// input.Messages reach the actual model call (previously the final closure
// captured the original history/opts and ignored middleware edits).
func TestRunModel_ReasoningMiddlewareMutationPropagates(t *testing.T) {
	rec := &recordingModel{name: "rec"}
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, nil, &forceNoneReasoningMW{}),
		chatModel: rec,
		memory:    memory.NewInMemoryMemory(),
	}
	hist := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}
	if _, err := a.runModel(context.Background(), hist, nil, 0, true); err != nil {
		t.Fatal(err)
	}
	// tool_choice=none must reach the model.
	var co model.ChatOptions
	for _, o := range rec.gotOpts {
		o(&co)
	}
	if co.ToolChoice == nil || co.ToolChoice.Mode != "none" {
		t.Fatalf("expected tool_choice=none to propagate to the model, got %+v", co.ToolChoice)
	}
	// The injected marker message must reach the model.
	found := false
	for _, m := range rec.gotMsgs {
		if m.GetTextContent() == "INJECTED-MARKER" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the middleware-injected message to reach the model")
	}
}

// TestInvokeModelChat_ModelCallMiddlewareMutationPropagates verifies the same
// fix at the on_model_call layer: a ModelCall interceptor's ChatOpts mutation
// reaches the Chat call.
type overrideFormatModelMW struct{ middleware.Base }

func (m *overrideFormatModelMW) OnModelCall(ctx context.Context, ag middleware.Agent, input *middleware.ModelCallInput, next middleware.ModelCallNext) (*message.Msg, error) {
	input.ChatOpts = append(input.ChatOpts, model.WithTemperature(0.42))
	return next(ctx)
}

func TestInvokeModelChat_ModelCallMiddlewareMutationPropagates(t *testing.T) {
	rec := &recordingModel{name: "rec"}
	a := &ReActAgent{
		Base:      agent.NewBase("", "t", "", "", nil, nil, nil, &overrideFormatModelMW{}),
		chatModel: rec,
		memory:    memory.NewInMemoryMemory(),
	}
	hist := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}
	if _, err := a.invokeModelChat(context.Background(), hist, nil, 0); err != nil {
		t.Fatal(err)
	}
	var co model.ChatOptions
	for _, o := range rec.gotOpts {
		o(&co)
	}
	if co.Temperature != 0.42 {
		t.Fatalf("expected temperature=0.42 from model-call middleware to propagate, got %v", co.Temperature)
	}
}
