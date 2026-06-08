package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/tool"
)

type stubAgent struct{ name string }

func (s stubAgent) AgentName() string { return s.name }

type replyMW struct {
	middleware.Base
	before bool
	after  bool
}

func (m *replyMW) OnReply(ctx context.Context, agent middleware.Agent, input *middleware.ReplyInput, next middleware.ReplyNext) (*message.Msg, error) {
	m.before = true
	msg, err := next(ctx)
	m.after = err == nil
	return msg, err
}

type systemMW struct {
	middleware.Base
	suffix string
}

func (m *systemMW) OnSystemPrompt(ctx context.Context, agent middleware.Agent, currentPrompt string) (string, error) {
	return currentPrompt + m.suffix, nil
}

type actingMW struct {
	middleware.Base
	seen bool
}

func (m *actingMW) OnActing(ctx context.Context, agent middleware.Agent, input *middleware.ActingInput, next middleware.ActingNext) (*tool.Response, error) {
	m.seen = input.ToolName == "echo"
	return next(ctx)
}

func TestClassifyAndChainReply(t *testing.T) {
	rmw := &replyMW{}
	chain := middleware.Classify([]middleware.Middleware{rmw, &systemMW{suffix: "-x"}})
	if len(chain.Reply) != 1 || len(chain.SystemPrompt) != 1 {
		t.Fatalf("unexpected chain: reply=%d system=%d", len(chain.Reply), len(chain.SystemPrompt))
	}

	agent := stubAgent{name: "a1"}
	input := &middleware.ReplyInput{Messages: []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}}
	handler := middleware.ChainReply(chain, agent, input, func(ctx context.Context) (*message.Msg, error) {
		return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
	})
	msg, err := handler(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "ok" || !rmw.before || !rmw.after {
		t.Fatalf("reply middleware not invoked: %+v before=%v after=%v", msg.GetTextContent(), rmw.before, rmw.after)
	}
}

func TestApplySystemPrompt(t *testing.T) {
	chain := middleware.Classify([]middleware.Middleware{&systemMW{suffix: " [skills]"}})
	out, err := middleware.ApplySystemPrompt(context.Background(), stubAgent{name: "a"}, chain, "base")
	if err != nil || out != "base [skills]" {
		t.Fatalf("got %q err=%v", out, err)
	}
}

func TestChainActing(t *testing.T) {
	amw := &actingMW{}
	chain := middleware.Classify([]middleware.Middleware{amw})
	handler := middleware.ChainActing(chain, stubAgent{name: "a"}, &middleware.ActingInput{ToolName: "echo"}, func(ctx context.Context) (*tool.Response, error) {
		return &tool.Response{}, nil
	})
	if _, err := handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !amw.seen {
		t.Fatal("acting middleware not invoked")
	}
}

func TestChainReplyPropagatesError(t *testing.T) {
	want := errors.New("boom")
	chain := middleware.Classify([]middleware.Middleware{&replyMW{}})
	handler := middleware.ChainReply(chain, stubAgent{name: "a"}, &middleware.ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return nil, want
	})
	_, err := handler(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("expected %v got %v", want, err)
	}
}
