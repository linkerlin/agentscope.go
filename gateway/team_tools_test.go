package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/linkerlin/agentscope.go/service"
	"github.com/linkerlin/agentscope.go/tool"
)

func teamTestSetup(t *testing.T) (*service.MemoryStorage, *messagebus.LocalBus, *TeamToolDeps) {
	t.Helper()
	storage := service.NewMemoryStorage()
	bus := messagebus.NewLocalBus()
	t.Cleanup(func() { _ = bus.Close() })
	return storage, bus, &TeamToolDeps{Storage: storage, Bus: bus}
}

func seedLeader(t *testing.T, storage *service.MemoryStorage) TeamToolContext {
	t.Helper()
	ctx := context.Background()
	_ = storage.SaveAgentConfig(ctx, &service.AgentConfig{ID: "agent-leader", UserID: "u1", Name: "Leader", ModelID: "openai/gpt-4"})
	_ = storage.SaveSession(ctx, &service.Session{ID: "sess-leader", UserID: "u1", AgentID: "agent-leader"})
	return TeamToolContext{UserID: "u1", AgentID: "agent-leader", SessionID: "sess-leader"}
}

func findTeamTool(tools []tool.Tool, name string) tool.Tool {
	for _, t := range tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

func TestNewTeamTools_Roles(t *testing.T) {
	_, _, deps := teamTestSetup(t)
	tctx := TeamToolContext{UserID: "u1", AgentID: "a", SessionID: "s"}

	if leader := newTeamTools("leader", tctx, deps); len(leader) != 4 {
		t.Fatalf("leader should have 4 tools, got %d", len(leader))
	}
	if worker := newTeamTools("worker", tctx, deps); len(worker) != 1 || worker[0].Name() != "TeamSay" {
		t.Fatalf("worker should have only TeamSay, got %v", worker)
	}
	if got := newTeamTools("leader", tctx, nil); got != nil {
		t.Fatalf("nil deps should yield no tools, got %v", got)
	}
}

func TestTeamCreate(t *testing.T) {
	storage, _, deps := teamTestSetup(t)
	tctx := seedLeader(t, storage)
	ctx := context.Background()

	tc := findTeamTool(newTeamTools("leader", tctx, deps), "TeamCreate")
	resp, err := tc.Execute(ctx, map[string]any{"name": "Research", "description": "does research"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "created") {
		t.Fatalf("expected created, got %q", resp.GetTextContent())
	}
	team, err := storage.GetTeamByLeaderSession(ctx, "sess-leader")
	if err != nil {
		t.Fatal(err)
	}
	if team.Name != "Research" {
		t.Fatalf("team name = %q", team.Name)
	}
	se, _ := storage.GetSession(ctx, "sess-leader")
	if se.TeamID != team.ID {
		t.Fatalf("leader session team_id not set")
	}

	// Idempotent reject: same session cannot lead two teams.
	resp2, _ := tc.Execute(ctx, map[string]any{"name": "Other"})
	if !strings.Contains(resp2.GetTextContent(), "already leads") {
		t.Fatalf("expected reject, got %q", resp2.GetTextContent())
	}
}

func TestAgentCreate(t *testing.T) {
	storage, bus, deps := teamTestSetup(t)
	tctx := seedLeader(t, storage)
	ctx := context.Background()
	tools := newTeamTools("leader", tctx, deps)
	findTeamTool(tools, "TeamCreate").Execute(ctx, map[string]any{"name": "T"})
	team, _ := storage.GetTeamByLeaderSession(ctx, "sess-leader")

	wakeCh, wakeCancel, _ := bus.SubscribeWakeup(ctx)
	defer wakeCancel()

	ac := findTeamTool(tools, "AgentCreate")
	resp, err := ac.Execute(ctx, map[string]any{"name": "researcher", "system_prompt": "you research", "prompt": "find X"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "created and started") {
		t.Fatalf("expected worker started, got %q", resp.GetTextContent())
	}

	team2, _ := storage.GetTeam(ctx, team.ID)
	if len(team2.Members) != 1 || team2.Members[0].Name != "researcher" {
		t.Fatalf("member not recorded: %+v", team2.Members)
	}
	wc, _ := storage.GetAgentConfig(ctx, team2.Members[0].AgentID)
	if wc.Source != "team" {
		t.Fatalf("worker source = %q", wc.Source)
	}
	ws, _ := storage.GetSession(ctx, team2.Members[0].SessionID)
	if ws.Source != "team" || ws.TeamID != team.ID {
		t.Fatalf("worker session wrong: %+v", ws)
	}
	// Inbox holds the initial prompt.
	msgs, _ := bus.InboxDrain(ctx, team2.Members[0].SessionID)
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "find X") {
		t.Fatalf("inbox wrong: %+v", msgs)
	}
	// Wakeup was enqueued for the worker session.
	select {
	case ev := <-wakeCh:
		if ev.SessionID != team2.Members[0].SessionID {
			t.Fatalf("wakeup session mismatch: %s", ev.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("no wakeup received")
	}

	// Name uniqueness within team.
	resp2, _ := ac.Execute(ctx, map[string]any{"name": "researcher", "prompt": "dup"})
	if !strings.Contains(resp2.GetTextContent(), "already exists") {
		t.Fatalf("expected duplicate reject, got %q", resp2.GetTextContent())
	}
}

func TestTeamSay(t *testing.T) {
	storage, bus, deps := teamTestSetup(t)
	tctx := seedLeader(t, storage)
	ctx := context.Background()
	tools := newTeamTools("leader", tctx, deps)
	findTeamTool(tools, "TeamCreate").Execute(ctx, map[string]any{"name": "T"})
	findTeamTool(tools, "AgentCreate").Execute(ctx, map[string]any{"name": "worker1", "prompt": "go"})
	team, _ := storage.GetTeamByLeaderSession(ctx, "sess-leader")
	workerSession := team.Members[0].SessionID
	_, _ = bus.InboxDrain(ctx, workerSession) // clear initial prompt from AgentCreate

	ts := findTeamTool(tools, "TeamSay")
	resp, err := ts.Execute(ctx, map[string]any{"content": "hello", "to": "worker1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "delivered to 1") {
		t.Fatalf("expected 1 recipient, got %q", resp.GetTextContent())
	}
	msgs, _ := bus.InboxDrain(ctx, workerSession)
	if len(msgs) != 1 || msgs[0].Content != "hello" || msgs[0].From != "Leader" {
		t.Fatalf("team message wrong: %+v", msgs)
	}

	// Broadcast (omit "to").
	ts.Execute(ctx, map[string]any{"content": "team update"})
	msgs2, _ := bus.InboxDrain(ctx, workerSession)
	if len(msgs2) != 1 || msgs2[0].Content != "team update" {
		t.Fatalf("broadcast wrong: %+v", msgs2)
	}

	// Unknown recipient.
	resp3, _ := ts.Execute(ctx, map[string]any{"content": "hi", "to": "nobody"})
	if !strings.Contains(resp3.GetTextContent(), "no teammate named") {
		t.Fatalf("expected no-match, got %q", resp3.GetTextContent())
	}
}

func TestTeamDelete(t *testing.T) {
	storage, _, deps := teamTestSetup(t)
	tctx := seedLeader(t, storage)
	ctx := context.Background()
	tools := newTeamTools("leader", tctx, deps)
	findTeamTool(tools, "TeamCreate").Execute(ctx, map[string]any{"name": "T"})
	findTeamTool(tools, "AgentCreate").Execute(ctx, map[string]any{"name": "w1", "prompt": "go"})
	team, _ := storage.GetTeamByLeaderSession(ctx, "sess-leader")
	workerAgent := team.Members[0].AgentID
	workerSession := team.Members[0].SessionID

	td := findTeamTool(tools, "TeamDelete")
	resp, err := td.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "deleted") {
		t.Fatalf("got %q", resp.GetTextContent())
	}
	if _, err := storage.GetTeam(ctx, team.ID); err == nil {
		t.Fatal("team should be deleted")
	}
	se, _ := storage.GetSession(ctx, "sess-leader")
	if se.TeamID != "" {
		t.Fatalf("leader team_id not cleared")
	}
	if _, err := storage.GetAgentConfig(ctx, workerAgent); err == nil {
		t.Fatal("worker config should be deleted")
	}
	if _, err := storage.GetSession(ctx, workerSession); err == nil {
		t.Fatal("worker session should be deleted")
	}
}
