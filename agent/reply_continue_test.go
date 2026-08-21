package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
)

// continueOnceMW returns ErrContinueReply on its first invocation and lets the
// reply pass on the second.
type continueOnceMW struct {
	middleware.Base
	calls int
	seen  [][]*message.Msg // input.Messages snapshots per round
}

func (m *continueOnceMW) OnReply(ctx context.Context, ag middleware.Agent, input *middleware.ReplyInput, next middleware.ReplyNext) (*message.Msg, error) {
	m.calls++
	snapshot := make([]*message.Msg, len(input.Messages))
	copy(snapshot, input.Messages)
	m.seen = append(m.seen, snapshot)
	resp, err := next(ctx)
	if err != nil {
		return nil, err
	}
	if m.calls == 1 {
		return resp, middleware.ErrContinueReply
	}
	return resp, nil
}

func TestBase_Call_ContinueReplyLoop(t *testing.T) {
	b := NewBase("t", "t", "", "", nil, nil, nil, &continueOnceMW{})

	replyCalls := 0
	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		replyCalls++
		return message.NewMsg().Role(message.RoleAssistant).TextContent("answer").Build(), nil
	}

	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	resp, err := b.Call(context.Background(), userMsg, reply)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.GetTextContent() != "answer" {
		t.Fatalf("unexpected reply: %q", resp.GetTextContent())
	}
	if replyCalls != 2 {
		t.Fatalf("reply should run twice (continue once), ran %d", replyCalls)
	}
}

// alwaysContinueMW never accepts a reply — the loop must be bounded.
type alwaysContinueMW struct {
	middleware.Base
	calls int
}

func (m *alwaysContinueMW) OnReply(ctx context.Context, ag middleware.Agent, input *middleware.ReplyInput, next middleware.ReplyNext) (*message.Msg, error) {
	m.calls++
	resp, err := next(ctx)
	if err != nil {
		return nil, err
	}
	return resp, middleware.ErrContinueReply
}

func TestBase_Call_ContinueLoopIsBounded(t *testing.T) {
	mw := &alwaysContinueMW{}
	b := NewBase("t", "t", "", "", nil, nil, nil, mw)

	replyCalls := 0
	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		replyCalls++
		return message.NewMsg().Role(message.RoleAssistant).TextContent("r").Build(), nil
	}
	resp, err := b.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), reply)
	if err != nil {
		t.Fatalf("bounded loop must not error when a reply exists: %v", err)
	}
	if resp == nil || resp.GetTextContent() != "r" {
		t.Fatalf("last reply should win, got %+v", resp)
	}
	if replyCalls != 4 { // initial + 3 continue rounds
		t.Fatalf("expected 4 reply runs (bounded), ran %d", replyCalls)
	}
}

var errBoom = errors.New("boom")

func TestBase_Call_PassThroughError(t *testing.T) {
	b := NewBase("t", "t", "", "", nil, nil, nil)
	reply := func(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
		return nil, errBoom
	}
	_, err := b.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), reply)
	if !errors.Is(err, errBoom) && err == nil {
		t.Fatalf("error must propagate, got %v", err)
	}
}
