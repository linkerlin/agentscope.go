// Command controlplane_sql demonstrates the evolved control-plane features at
// the library level: SQL durability across restarts, cross-process writeback→
// spend, ticket enforcement with turn tokens, capability lane-gate enforcement
// (issue-fix "merge" requires an approved gate), reward-memory hard_policy
// vetoes, and maintenance reaping. Run:
//
//	go run ./examples/controlplane_sql
//
// Everything runs against a local SQLite file (zero-CGO via modernc.org/sqlite),
// so state survives process restarts — no external services required.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // register the sqlite driver (zero-CGO)

	"github.com/linkerlin/agentscope.go/controlplane"
)

func main() {
	ctx := context.Background()
	dbPath := filepath.Join(os.TempDir(), "cp-example.sqlite")
	// Self-clean: a prior run may have left active rewards/gates that would
	// change this run's ShouldRun decisions (e.g. a stale hard_policy veto).
	_ = os.Remove(dbPath)

	// --- 1. SQL durability: state survives a full close+reopen ---
	fmt.Println("== 1. SQL durability across restart ==")
	writePhase(ctx, dbPath)
	readPhase(ctx, dbPath)

	// --- 2. Cross-process: authorize/writeback in one Kernel, spend in another ---
	fmt.Println("\n== 2. Cross-process writeback -> spend (shared DB) ==")
	dbA, dbB := openDB(dbPath), openDB(dbPath)
	kA, _ := controlplane.NewSQLKernel(dbA)
	kB, _ := controlplane.NewSQLKernel(dbB)
	kA.WithTicketEnforcement() // enforcement is a per-Kernel flag; enable on the spender too
	kB.WithTicketEnforcement()

	dec, err := kA.ShouldRunTurn(ctx, "goal-1", "worker-a", "turn-1")
	must(err)
	fmt.Printf("  A authorized turn: should_run=%v token=%s…\n", dec.ShouldRun, dec.TurnToken[:8])
	_, err = kA.Writeback(ctx, controlplane.ValidatedWriteback{
		GoalID: "goal-1", TodoID: "todo-1", TurnID: "turn-1", AgentID: "worker-a",
		Outcome:  controlplane.Outcome{Status: controlplane.OutcomeProgress},
		Evidence: []controlplane.Evidence{{ID: "ev-1", Kind: "diff", Summary: "implementation diff"}},
	})
	must(err)
	fmt.Println("  A writeback ok (delivery recorded in shared store)")
	total, err := kB.SpendSlot(ctx, "goal-1", "turn-1", controlplane.SpendOpts{Execute: true, Token: dec.TurnToken})
	must(err)
	fmt.Printf("  B spent A's delivery (cross-process): spent_in_window=%d\n", total)

	// --- 3. Ticket enforcement: wrong/missing token rejected ---
	fmt.Println("\n== 3. Ticket enforcement (stale/wrong token rejected) ==")
	dec2, _ := kA.ShouldRunTurn(ctx, "goal-1", "worker-a", "turn-2")
	_, err = kA.Writeback(ctx, controlplane.ValidatedWriteback{
		GoalID: "goal-1", TodoID: "todo-1", TurnID: "turn-2", AgentID: "worker-a",
		Outcome:  controlplane.Outcome{Status: controlplane.OutcomeProgress},
		Evidence: []controlplane.Evidence{{ID: "ev-2", Kind: "diff", Summary: "ok"}},
	})
	must(err)
	_, err = kA.SpendSlot(ctx, "goal-1", "turn-2", controlplane.SpendOpts{Execute: true, Token: "bogus"})
	fmt.Printf("  wrong token: %v\n", err)
	// Gate opens mid-turn -> the minted ticket is now STALE.
	must(kA.OpenGate(ctx, controlplane.UserGate{
		GateID: "g-mid", GoalID: "goal-1", Question: "hold for review",
		Scope: controlplane.DecisionScope{Kind: controlplane.ScopeWrite, ScopeKey: "refs/heads/main"},
	}))
	_, err = kA.SpendSlot(ctx, "goal-1", "turn-2", controlplane.SpendOpts{Execute: true, Token: dec2.TurnToken})
	fmt.Printf("  stale ticket (gate opened mid-turn): %v\n", err)
	must(kA.ResolveGate(ctx, "goal-1", "g-mid", controlplane.GateOutcome{Decision: controlplane.DecisionApprove, By: "lead"}))

	// --- 4. Capability lane gate: issue-fix "merge" needs an approved gate ---
	fmt.Println("\n== 4. Capability lane gate (issue-fix merge stage) ==")
	g, _ := kA.GoalStore().Get(ctx, "goal-1")
	g.CapabilityID = "issue-fix"
	must(kA.GoalStore().Upsert(ctx, g))
	must(kA.TodoStore().Upsert(ctx, &controlplane.Todo{
		ID: "todo-merge", GoalID: "goal-1", StageID: "merge",
		Description: "merge the fix", TaskClass: controlplane.TaskAdvancement, State: controlplane.TodoOpen,
	}))
	_, err = kA.Writeback(ctx, controlplane.ValidatedWriteback{
		GoalID: "goal-1", TodoID: "todo-merge", TurnID: "turn-merge", AgentID: "worker-a",
		Outcome:  controlplane.Outcome{Status: controlplane.OutcomeCompletion},
		Evidence: []controlplane.Evidence{{ID: "ev-merge", Kind: "merge", Summary: "merged"}},
	})
	fmt.Printf("  merge without approved gate: %v\n", err) // ErrLaneGateRequired
	must(kA.OpenGate(ctx, controlplane.UserGate{
		GateID: "g-merge", GoalID: "goal-1", TodoID: "todo-merge", Question: "approve merge?",
		Scope: controlplane.DecisionScope{Kind: controlplane.ScopePublicClaim, ScopeKey: "main"},
	}))
	must(kA.ResolveGate(ctx, "goal-1", "g-merge", controlplane.GateOutcome{Decision: controlplane.DecisionApprove, By: "lead"}))
	_, err = kA.Writeback(ctx, controlplane.ValidatedWriteback{
		GoalID: "goal-1", TodoID: "todo-merge", TurnID: "turn-merge2", AgentID: "worker-a",
		Outcome:  controlplane.Outcome{Status: controlplane.OutcomeCompletion},
		Evidence: []controlplane.Evidence{{ID: "ev-merge2", Kind: "merge", Summary: "merged"}},
	})
	must(err)
	fmt.Println("  merge with approved gate: ok")

	// --- 5. Reward hard_policy veto ---
	fmt.Println("\n== 5. Reward hard_policy veto ==")
	must(kA.RecordReward(ctx, "goal-1", controlplane.RewardRecord{
		Class: controlplane.AuthorityHardPolicy, Lifecycle: controlplane.LifecycleActive,
		Scope:   controlplane.DecisionScope{Kind: controlplane.ScopeProduction, Granularity: controlplane.GranularityGoal, ScopeKey: "goal-1"},
		Content: "no deploys until audit clears",
	}))
	dec3, _ := kA.ShouldRun(ctx, "goal-1", "worker-a")
	fmt.Printf("  ShouldRun under hard_policy: should_run=%v state=%s reason=%s\n", dec3.ShouldRun, dec3.State, dec3.Reason)

	// --- 6. Maintenance: reap + compact via ReapAll ---
	fmt.Println("\n== 6. Maintenance (ReapAll) ==")
	must(kA.ReapAll(ctx, time.Hour, 7*24*time.Hour, 200))
	fmt.Println("  reaped consumed tickets/deliveries/inactive rewards, compacted ledgers")

	// --- 7. Review packet: active policies + recent lineage surfaced ---
	fmt.Println("\n== 7. Operator review packet ==")
	pkt, err := kA.ReviewPacket(ctx, "goal-1")
	must(err)
	fmt.Printf("  goal=%s state=%s | open todos=%d | pending gates=%d | active policies=%d\n",
		pkt.Goal.ID, pkt.Goal.State, len(pkt.OpenTodos), len(pkt.PendingGates), len(pkt.ActivePolicies))
	for _, p := range pkt.ActivePolicies {
		fmt.Printf("    policy [%s] %s\n", p.Class, p.Content)
	}
	fmt.Printf("  lineage events: %d\n", len(pkt.DecisionLineage))

	dbA.Close()
	dbB.Close()
	fmt.Println("\nAll examples completed.")
}

func writePhase(ctx context.Context, dbPath string) {
	db := openDB(dbPath)
	k, err := controlplane.NewSQLKernel(db)
	must(err)
	must(k.GoalStore().Upsert(ctx, &controlplane.Goal{
		ID: "goal-1", OwnerUserID: "alice", Objective: "Ship feature X safely",
		State: controlplane.GoalActive, Quota: controlplane.DefaultQuota(), CurrentTodoID: "todo-1",
	}))
	must(k.TodoStore().Upsert(ctx, &controlplane.Todo{
		ID: "todo-1", GoalID: "goal-1", Description: "implement with tests",
		TaskClass: controlplane.TaskAdvancement, State: controlplane.TodoOpen,
	}))
	db.Close()
	fmt.Printf("  wrote goal+todo to %s\n", filepath.Base(dbPath))
}

func readPhase(ctx context.Context, dbPath string) {
	db := openDB(dbPath)
	k, err := controlplane.NewSQLKernel(db)
	must(err)
	g, err := k.GoalStore().Get(ctx, "goal-1")
	must(err)
	fmt.Printf("  reopened: goal=%q objective=%q state=%s current_todo=%s\n",
		g.ID, g.Objective, g.State, g.CurrentTodoID)
	db.Close()
}

func openDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	must(err)
	return db
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
