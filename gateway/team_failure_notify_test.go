package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// failingV2Agent emits a terminal ErrorEvent for every turn.
type failingV2Agent struct{}

func (failingV2Agent) Name() string { return "failing" }

func (failingV2Agent) Call(context.Context, *message.Msg) (*message.Msg, error) {
	return nil, nil
}

func (failingV2Agent) CallStream(context.Context, *message.Msg) (<-chan *message.Msg, error) {
	return nil, nil
}

func (failingV2Agent) Reply(context.Context, *message.Msg) (*message.Msg, error) {
	return nil, nil
}

func (failingV2Agent) ReplyStream(_ context.Context, _ *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 2)
	ch <- event.NewError("r1", context.DeadlineExceeded)
	ch <- event.NewReplyEnd("r1", "failing")
	close(ch)
	return ch, nil
}

func (failingV2Agent) LoadState(*agent.AgentState) error     { return nil }
func (failingV2Agent) SaveState() (*agent.AgentState, error) { return nil, nil }
func (failingV2Agent) InjectEvent(context.Context, event.AgentEvent) error {
	return nil
}

func TestWakeupDispatcher_WorkerFailureNotifiesLeader(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	storage := service.NewMemoryStorage()
	if err := storage.SaveTeam(ctx, &service.Team{
		ID: "team-1", UserID: "u1", LeaderSessionID: "leader-s",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveSession(ctx, &service.Session{
		ID: "worker-s", UserID: "u1", AgentID: "worker", TeamID: "team-1",
		Source: "team", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	bus := messagebus.NewLocalBus()
	d := NewWakeupDispatcher(bus, NewSessionManager(), storage,
		func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
			return failingV2Agent{}, nil
		})

	// Seed the worker inbox, then drain+run: the failing turn must surface in
	// the leader's inbox as a <team-error> message.
	if err := bus.InboxPush(ctx, "worker-s", messagebus.TeamMessage{From: "leader", Content: "do work"}); err != nil {
		t.Fatal(err)
	}
	d.drainAndRun(ctx, "worker-s")

	deadline := time.Now().Add(2 * time.Second)
	for {
		msgs, _ := bus.InboxDrain(ctx, "leader-s")
		for _, m := range msgs {
			if m.From == "worker:worker-s" && strings.HasPrefix(m.Content, "<team-error") {
				return // notified
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("leader was not notified of the worker failure")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
