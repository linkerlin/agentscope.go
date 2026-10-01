package gateway

import (
	"context"
	"strings"
	"testing"
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
