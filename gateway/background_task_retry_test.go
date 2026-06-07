package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/schedule"
)

type countingAgent struct {
	calls int
	fail  int
}

func (a *countingAgent) Name() string { return "counter" }

func (a *countingAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	a.calls++
	if a.calls <= a.fail {
		return nil, errors.New("boom")
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (a *countingAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	resp, err := a.Call(ctx, msg)
	ch := make(chan *message.Msg, 1)
	if err == nil {
		ch <- resp
	}
	close(ch)
	return ch, err
}

func TestBackgroundTaskManager_RetryOnFailure(t *testing.T) {
	ag := &countingAgent{fail: 2}
	reg := NewAgentRegistry()
	reg.Register("counter", ag)

	btm := NewBackgroundTaskManager(reg, nil)
	job := &schedule.Job{
		ID:         "j1",
		AgentID:    "counter",
		Payload:    "hi",
		MaxRetries: 3,
		RetryDelay: 5 * time.Millisecond,
	}
	if err := btm.handle(context.Background(), job); err != nil {
		t.Fatalf("expected success after retries, got %v (calls=%d)", err, ag.calls)
	}
	if ag.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", ag.calls)
	}
}

var _ agent.Agent = (*countingAgent)(nil)
