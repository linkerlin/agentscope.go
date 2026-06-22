package gateway

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
)

// TestTeamTools_EdgeCases covers the precondition self-checks each tool
// performs at call time (mirrors Python's per-call validation).
func TestTeamTools_EdgeCases(t *testing.T) {
	storage, _, deps := teamTestSetup(t)
	tctx := seedLeader(t, storage)
	ctx := context.Background()
	tools := newTeamTools("leader", tctx, deps)

	ac := findTeamTool(tools, "AgentCreate")
	ts := findTeamTool(tools, "TeamSay")
	td := findTeamTool(tools, "TeamDelete")
	tc := findTeamTool(tools, "TeamCreate")

	// Before any team exists: create/say/delete all refuse.
	if r, _ := ac.Execute(ctx, map[string]any{"name": "w", "prompt": "p"}); !strings.Contains(r.GetTextContent(), "does not lead any team") {
		t.Fatalf("AgentCreate without team: got %q", r.GetTextContent())
	}
	if r, _ := ts.Execute(ctx, map[string]any{"content": "hi"}); !strings.Contains(r.GetTextContent(), "not in any team") {
		t.Fatalf("TeamSay without team: got %q", r.GetTextContent())
	}
	if r, _ := td.Execute(ctx, map[string]any{}); !strings.Contains(r.GetTextContent(), "does not lead any team") {
		t.Fatalf("TeamDelete without team: got %q", r.GetTextContent())
	}

	// TeamCreate without name.
	if r, _ := tc.Execute(ctx, map[string]any{}); !strings.Contains(r.GetTextContent(), "required") {
		t.Fatalf("TeamCreate without name: got %q", r.GetTextContent())
	}

	// Now create a team; AgentCreate still validates its own args.
	if _, err := tc.Execute(ctx, map[string]any{"name": "T"}); err != nil {
		t.Fatal(err)
	}
	if r, _ := ac.Execute(ctx, map[string]any{"name": "w"}); !strings.Contains(r.GetTextContent(), "required") {
		t.Fatalf("AgentCreate without prompt: got %q", r.GetTextContent())
	}
	if r, _ := ac.Execute(ctx, map[string]any{"prompt": "p"}); !strings.Contains(r.GetTextContent(), "required") {
		t.Fatalf("AgentCreate without name: got %q", r.GetTextContent())
	}
}

// TestTeamTools_WorkerReportsToLeader verifies the full bidirectional loop:
// a worker (source="team") gets ONLY TeamSay, and can route a message back to
// the leader by name, landing in the leader session's inbox.
func TestTeamTools_WorkerReportsToLeader(t *testing.T) {
	storage, bus, deps := teamTestSetup(t)
	leaderCtx := seedLeader(t, storage)
	ctx := context.Background()

	// Leader sets up team + worker.
	tools := newTeamTools("leader", leaderCtx, deps)
	findTeamTool(tools, "TeamCreate").Execute(ctx, map[string]any{"name": "T"})
	findTeamTool(tools, "AgentCreate").Execute(ctx, map[string]any{"name": "researcher", "prompt": "go"})
	team, _ := storage.GetTeamByLeaderSession(ctx, "sess-leader")
	worker := team.Members[0]
	// Clear the initial prompt AgentCreate delivered to the worker.
	_, _ = bus.InboxDrain(ctx, worker.SessionID)

	// Worker's tool set: only TeamSay.
	workerCtx := TeamToolContext{UserID: "u1", AgentID: worker.AgentID, SessionID: worker.SessionID}
	workerTools := newTeamTools("worker", workerCtx, deps)
	if len(workerTools) != 1 || workerTools[0].Name() != "TeamSay" {
		t.Fatalf("worker should have only TeamSay, got %v", workerTools)
	}

	// Worker reports completion to the leader (addressed by name).
	resp, err := workerTools[0].Execute(ctx, map[string]any{"content": "task done", "to": "Leader"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "delivered to 1") {
		t.Fatalf("expected delivery to leader, got %q", resp.GetTextContent())
	}

	// Leader session inbox received it, attributed to the worker name.
	msgs, _ := bus.InboxDrain(ctx, "sess-leader")
	if len(msgs) != 1 || msgs[0].Content != "task done" || msgs[0].From != "researcher" {
		t.Fatalf("leader inbox wrong: %+v", msgs)
	}

	// Self-send is skipped: worker broadcasting should not deliver to itself.
	workerTools[0].Execute(ctx, map[string]any{"content": "note to self"})
	if m, _ := bus.InboxDrain(ctx, worker.SessionID); len(m) != 0 {
		t.Fatalf("worker should not receive its own broadcast, got %+v", m)
	}
}

// holdV2Agent keeps its ReplyStream open until the channel is closed, letting a
// test hold a session "active" and then release it.
type holdV2Agent struct {
	fakeV2Agent
	stream chan event.AgentEvent
}

func (h *holdV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	return h.stream, nil
}

// TestWakeupDispatcher_BusyRetry verifies the busy path: when a wakeup targets
// a session that is already running, the dispatcher waits for it to become idle
// (polling), then drains the inbox and runs. Messages are never lost.
func TestWakeupDispatcher_BusyRetry(t *testing.T) {
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()

	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "wa", UserID: "u1", Name: "W", Source: "team"})
	_ = storage.SaveSession(ctx, &service.Session{ID: "ws", UserID: "u1", AgentID: "wa", Source: "team"})
	_ = bus.InboxPush(ctx, "ws", messagebus.TeamMessage{From: "L", Content: "do task"})

	sm := NewSessionManager()
	// Occupy the session with a holder whose stream never closes on its own.
	hold := &holdV2Agent{stream: make(chan event.AgentEvent)}
	holdMsg := message.NewMsg().Role(message.RoleUser).TextContent("hold").Build()
	if _, err := sm.Run(ctx, "ws", hold, holdMsg); err != nil {
		t.Fatal(err)
	}
	if !sm.IsActive("ws") {
		t.Fatal("session should be active while holder stream is open")
	}

	var buildCalled atomic.Bool
	d := NewWakeupDispatcher(bus, sm, storage, func(ctx context.Context, agentID, sessionID string) (agent.Agent, error) {
		buildCalled.Store(true)
		return &fakeV2Agent{}, nil
	})

	// Wakeup hits a busy session -> spawns a background retry; build must not
	// happen yet.
	d.handleWakeup(ctx, "ws")
	time.Sleep(300 * time.Millisecond)
	if buildCalled.Load() {
		t.Fatal("buildAgent must not run while the session is busy")
	}

	// Release the holder: closing the stream lets the run finish -> idle.
	close(hold.stream)

	// The retry loop should detect idle and process the inbox.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !buildCalled.Load() {
		time.Sleep(50 * time.Millisecond)
	}
	if !buildCalled.Load() {
		t.Fatal("buildAgent not called after session became idle (busy retry failed)")
	}
	if msgs, _ := bus.InboxDrain(ctx, "ws"); len(msgs) != 0 {
		t.Fatalf("inbox should be drained after busy retry, got %d", len(msgs))
	}
}
