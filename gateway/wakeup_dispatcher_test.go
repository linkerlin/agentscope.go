package gateway

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// fakeV2Agent is a no-op V2Agent whose ReplyStream closes immediately,
// letting SessionManager.Run finish instantly in tests.
type fakeV2Agent struct{}

func (f *fakeV2Agent) Name() string { return "fake" }
func (f *fakeV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return msg, nil
}
func (f *fakeV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg)
	close(ch)
	return ch, nil
}
func (f *fakeV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return msg, nil
}
func (f *fakeV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent)
	close(ch)
	return ch, nil
}
func (f *fakeV2Agent) LoadState(s *agent.AgentState) error  { return nil }
func (f *fakeV2Agent) SaveState() (*agent.AgentState, error) { return &agent.AgentState{}, nil }
func (f *fakeV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func TestWakeupDispatcher_DrainAndRun(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()

	// Seed a worker session + config (as AgentCreate would).
	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "wa", UserID: "u1", Name: "Worker", Source: "team"})
	_ = storage.SaveSession(ctx, &service.Session{ID: "ws", UserID: "u1", AgentID: "wa", Source: "team"})
	_ = bus.InboxPush(ctx, "ws", messagebus.TeamMessage{From: "Leader", Content: "do task"})

	buildCalled := false
	sm := NewSessionManager()
	d := NewWakeupDispatcher(bus, sm, storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		buildCalled = true
		if agentID != "wa" || sessionID != "ws" {
			t.Errorf("unexpected build args: agent=%s session=%s", agentID, sessionID)
		}
		return &fakeV2Agent{}, nil
	})

	d.drainAndRun(ctx, "ws")

	if !buildCalled {
		t.Fatal("buildAgent was not called")
	}
	// Inbox must be drained after the run is kicked off.
	if msgs, _ := bus.InboxDrain(ctx, "ws"); len(msgs) != 0 {
		t.Fatalf("inbox should be empty after drainAndRun, got %d", len(msgs))
	}
}

func TestWakeupDispatcher_OrphanGuard(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()

	buildCalled := false
	d := NewWakeupDispatcher(bus, NewSessionManager(), storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		buildCalled = true
		return &fakeV2Agent{}, nil
	})
	// No session seeded -> orphan guard should skip.
	d.drainAndRun(ctx, "nonexistent")
	if buildCalled {
		t.Fatal("buildAgent should not be called for orphan session")
	}
}
