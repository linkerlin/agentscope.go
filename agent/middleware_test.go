package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
)

type errorHook struct {
	seen bool
	err  error
}

func (h *errorHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if hCtx.Point == hook.HookOnError {
		h.seen = true
		h.err = hCtx.Err
		return &hook.HookResult{HandleError: true}, nil
	}
	return nil, nil
}

type replyMiddleware struct {
	middleware.Base
	called bool
}

func (m *replyMiddleware) OnReply(ctx context.Context, ag middleware.Agent, input *middleware.ReplyInput, next middleware.ReplyNext) (*message.Msg, error) {
	m.called = true
	return next(ctx)
}

func TestBase_FireOnError(t *testing.T) {
	h := &errorHook{}
	b := agent.NewBase("id", "name", "", "", nil, []hook.Hook{h}, nil)
	want := errors.New("fail")
	err := b.FireOnError(context.Background(), want, nil, "", nil)
	if err != nil {
		t.Fatalf("expected swallowed error, got %v", err)
	}
	if !h.seen || !errors.Is(h.err, want) {
		t.Fatalf("hook not fired: seen=%v err=%v", h.seen, h.err)
	}
}

func TestBase_Call_OnReplyMiddleware(t *testing.T) {
	mw := &replyMiddleware{}
	b := agent.NewBase("id", "name", "", "", nil, nil, nil, mw)
	resp, err := b.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
		func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
			return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
		})
	if err != nil || resp.GetTextContent() != "ok" || !mw.called {
		t.Fatalf("resp=%v err=%v called=%v", resp.GetTextContent(), err, mw.called)
	}
}

func TestBase_Call_FireOnError(t *testing.T) {
	h := &errorHook{}
	b := agent.NewBase("id", "name", "", "", nil, []hook.Hook{h}, nil)
	want := errors.New("reply failed")
	_, err := b.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
		func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
			return nil, want
		})
	if err != nil {
		t.Fatalf("expected swallowed error from hook, got %v", err)
	}
	if !h.seen {
		t.Fatal("on_error hook not fired")
	}
}
