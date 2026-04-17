package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// trackingHook records which points were fired and allows mutating messages.
type trackingHook struct {
	points         []hook.HookPoint
	injectOnPoint  hook.HookPoint
	overrideOnPoint hook.HookPoint
	interruptOnPoint hook.HookPoint
}

func (h *trackingHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	h.points = append(h.points, hCtx.Point)
	if hCtx.Point == h.injectOnPoint && len(hCtx.Messages) > 0 {
		modified := append([]*message.Msg(nil), hCtx.Messages...)
		modified[0] = message.NewMsg().
			Role(message.RoleUser).
			TextContent("injected").
			Build()
		return &hook.HookResult{InjectMessages: modified}, nil
	}
	if hCtx.Point == h.overrideOnPoint {
		return &hook.HookResult{Override: message.NewMsg().
			Role(message.RoleAssistant).
			TextContent("override").
			Build()}, nil
	}
	if hCtx.Point == h.interruptOnPoint {
		return &hook.HookResult{Interrupt: true, Override: message.NewMsg().
			Role(message.RoleAssistant).
			TextContent("interrupted").
			Build()}, nil
	}
	return nil, nil
}

func TestBase_Call_Lifecycle(t *testing.T) {
	h := &trackingHook{}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		return message.NewMsg().
			Role(message.RoleAssistant).
			TextContent("reply:" + msg.GetTextContent()).
			Build(), nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	resp, err := b.Call(context.Background(), input, reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "reply:hello" {
		t.Fatalf("expected reply:hello, got %s", resp.GetTextContent())
	}
	if len(h.points) != 2 || h.points[0] != hook.HookPreReply || h.points[1] != hook.HookPostReply {
		t.Fatalf("expected pre_reply then post_reply, got %v", h.points)
	}
}

func TestBase_Call_InjectMessage(t *testing.T) {
	h := &trackingHook{injectOnPoint: hook.HookPreReply}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	var received string
	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		received = msg.GetTextContent()
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	_, err := b.Call(context.Background(), input, reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received != "injected" {
		t.Fatalf("expected injected message, got %s", received)
	}
}

func TestBase_Call_Override(t *testing.T) {
	h := &trackingHook{overrideOnPoint: hook.HookPreReply}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("should not reach").Build(), nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	resp, err := b.Call(context.Background(), input, reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "override" {
		t.Fatalf("expected override, got %s", resp.GetTextContent())
	}
}

func TestBase_Call_Interrupt(t *testing.T) {
	h := &trackingHook{interruptOnPoint: hook.HookPreReply}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("should not reach").Build(), nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	resp, err := b.Call(context.Background(), input, reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTextContent() != "interrupted" {
		t.Fatalf("expected interrupted, got %s", resp.GetTextContent())
	}
}

func TestBase_Call_ReplyError(t *testing.T) {
	b := NewBase("id", "name", "desc", "sys", nil, nil, nil)
	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		return nil, errors.New("boom")
	}
	_, err := b.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), reply)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBase_Observe_Lifecycle(t *testing.T) {
	h := &trackingHook{}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	var observed string
	observe := func(ctx context.Context, msg *message.Msg) error {
		observed = msg.GetTextContent()
		return nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	err := b.Observe(context.Background(), input, observe)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observed != "hello" {
		t.Fatalf("expected hello, got %s", observed)
	}
	if len(h.points) != 2 || h.points[0] != hook.HookPreObserve || h.points[1] != hook.HookPostObserve {
		t.Fatalf("expected pre_observe then post_observe, got %v", h.points)
	}
}

func TestBase_Observe_Inject(t *testing.T) {
	h := &trackingHook{injectOnPoint: hook.HookPreObserve}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	var observed string
	observe := func(ctx context.Context, msg *message.Msg) error {
		observed = msg.GetTextContent()
		return nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	err := b.Observe(context.Background(), input, observe)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observed != "injected" {
		t.Fatalf("expected injected, got %s", observed)
	}
}

func TestBase_Observe_Interrupt(t *testing.T) {
	h := &trackingHook{interruptOnPoint: hook.HookPreObserve}
	b := NewBase("id", "name", "desc", "sys", nil, []hook.Hook{h}, nil)

	called := false
	observe := func(ctx context.Context, msg *message.Msg) error {
		called = true
		return nil
	}

	input := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	err := b.Observe(context.Background(), input, observe)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("expected observe not to be called due to interrupt")
	}
}

func TestBase_Observe_Error(t *testing.T) {
	b := NewBase("id", "name", "desc", "sys", nil, nil, nil)
	observe := func(ctx context.Context, msg *message.Msg) error {
		return errors.New("boom")
	}
	err := b.Observe(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), observe)
	if err == nil {
		t.Fatal("expected error")
	}
}


func TestBase_AgentName(t *testing.T) {
	b := NewBase("id", "agent-name", "", "", nil, nil, nil)
	if b.AgentName() != "agent-name" {
		t.Fatalf("expected agent-name, got %s", b.AgentName())
	}
}

func TestBase_Shutdown(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	ctx := context.Background()
	if err := b.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if !b.IsClosed() {
		t.Fatal("expected closed")
	}
}

func TestBase_Shutdown_WaitForCall(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	b.CallWg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		b.CallWg.Done()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := b.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestBase_Shutdown_Timeout(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	b.CallWg.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := b.Shutdown(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected timeout, got %v", err)
	}
	b.CallWg.Done() // clean up
}

func TestBase_Usage(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	if b.TotalUsage().TotalTokens != 0 {
		t.Fatal("expected zero usage")
	}
	b.AddUsage(model.ChatUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	b.AddUsage(model.ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30})
	u := b.TotalUsage()
	if u.PromptTokens != 11 || u.CompletionTokens != 22 || u.TotalTokens != 33 {
		t.Fatalf("unexpected usage: %+v", u)
	}
}

func TestBase_HasStreamHooks(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	if b.HasStreamHooks() {
		t.Fatal("expected no stream hooks")
	}
	b2 := NewBase("id", "name", "", "", nil, nil, []hook.StreamHook{hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) { return nil, nil })})
	if !b2.HasStreamHooks() {
		t.Fatal("expected stream hooks")
	}
}

func TestBase_FireStreamEvent(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	ev, res, err := b.FireStreamEvent(context.Background(), &hook.PreReasoningEvent{})
	if err != nil || res != nil {
		t.Fatalf("unexpected result: %v %v", res, err)
	}
	_ = ev

	// With stream hook that interrupts
	sh := hook.StreamHookFunc(func(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
		return &hook.StreamHookResult{Interrupt: true}, nil
	})
	b2 := NewBase("id", "name", "", "", nil, nil, []hook.StreamHook{sh})
	_, res2, err2 := b2.FireStreamEvent(context.Background(), &hook.PreReasoningEvent{})
	if err2 != hook.ErrInterrupted || res2 == nil || !res2.Interrupt {
		t.Fatalf("expected interrupt, got %v %v", res2, err2)
	}
}

func TestBase_Call_PostReplyNilResponse(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		return nil, nil
	}
	resp, err := b.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), reply)
	if err != nil || resp != nil {
		t.Fatalf("expected nil response without error")
	}
}

func TestBase_FireHooks_NoHooks(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	msgs, hr, err := b.FireHooks(context.Background(), hook.HookBeforeModel, []*message.Msg{}, nil, "", nil)
	if err != nil || hr != nil || len(msgs) != 0 {
		t.Fatalf("unexpected result")
	}
}

func TestBase_Interrupt(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)

	if b.IsInterrupted() {
		t.Fatal("expected not interrupted initially")
	}
	if err := b.CheckInterrupted(); err != nil {
		t.Fatalf("unexpected interrupt error: %v", err)
	}

	b.Interrupt()
	if !b.IsInterrupted() {
		t.Fatal("expected interrupted")
	}
	if err := b.CheckInterrupted(); err == nil {
		t.Fatal("expected interrupt error")
	}

	b.ResetInterrupt()
	if b.IsInterrupted() {
		t.Fatal("expected not interrupted after reset")
	}
}

func TestBase_InterruptWithMsg(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	msg := message.NewMsg().Role(message.RoleUser).TextContent("stop").Build()
	b.InterruptWithMsg(msg)

	if !b.IsInterrupted() {
		t.Fatal("expected interrupted")
	}
	ic := b.CreateInterruptContext(nil)
	if ic.Source != "USER" {
		t.Fatalf("expected USER source, got %s", ic.Source)
	}
	if ic.UserMessage == nil || ic.UserMessage.GetTextContent() != "stop" {
		t.Fatal("expected user message in context")
	}
}

func TestBase_InterruptWithSource(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	b.InterruptWithSource("SYSTEM")
	ic := b.CreateInterruptContext([]*message.ToolUseBlock{
		message.NewToolUseBlock("t1", "tool", map[string]any{}),
	})
	if ic.Source != "SYSTEM" {
		t.Fatalf("expected SYSTEM source, got %s", ic.Source)
	}
	if len(ic.PendingToolCalls) != 1 {
		t.Fatalf("expected 1 pending tool call, got %d", len(ic.PendingToolCalls))
	}
}

func TestBase_InterruptWithSource_DefaultsToSystem(t *testing.T) {
	b := NewBase("id", "name", "", "", nil, nil, nil)
	b.InterruptWithSource("")
	ic := b.CreateInterruptContext(nil)
	if ic.Source != "SYSTEM" {
		t.Fatalf("expected SYSTEM source for empty string, got %s", ic.Source)
	}
}
