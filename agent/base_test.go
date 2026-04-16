package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
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
