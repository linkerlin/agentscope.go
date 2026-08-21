// Command controlplane_demo demonstrates the control plane (LoopX-style
// governance) end to end: a durable lifetime goal, quota-gated ticks,
// evidence-backed writeback, a concrete user gate that blocks delivery, a
// scoped fallback that runs without bypassing the gate, hard peer leases with
// CAS contention, deterministic peer assignment, and an operator review packet.
//
// Run:
//
//	go run ./examples/controlplane_demo
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/evolver"
)

func main() {
	ctx := context.Background()
	// Callers hold their own store references and pass them to NewKernel. This
	// keeps the Kernel free of store accessors and matches real usage.
	goals := controlplane.NewMemoryGoalStore()
	todos := controlplane.NewMemoryTodoStore()
	k := controlplane.NewKernel(goals, todos, nil)

	// --- 1. Register a durable lifetime goal + a todo ---
	goal := &controlplane.Goal{
		ID: "goal-feature", Objective: "Ship feature X safely",
		Scope: []string{"no direct push to main", "tests must pass"},
		State: controlplane.GoalActive,
		Quota: controlplane.DefaultQuota(),
	}
	must(goals.Upsert(ctx, goal))
	must(todos.Upsert(ctx, &controlplane.Todo{
		ID: "todo-impl", GoalID: "goal-feature",
		Description: "implement feature X with tests", TaskClass: controlplane.TaskAdvancement,
		State: controlplane.TodoOpen, Order: 1,
	}))
	goal.CurrentTodoID = "todo-impl"
	must(goals.Upsert(ctx, goal))

	fmt.Println("== 1. ShouldRun (eligible) ==")
	dec, _ := k.ShouldRun(ctx, "goal-feature", "agent-1")
	printDecision(dec)

	fmt.Println("\n== 2. Tick claims the current todo ==")
	res, _ := k.Tick(ctx, "goal-feature", "agent-1", "turn-1")
	fmt.Printf("  claimed=%s\n", res.ClaimedTodoID)

	fmt.Println("\n== 3. Runtime executes work, then validated writeback + spend ==")
	deliveryID, err := k.Writeback(ctx, controlplane.ValidatedWriteback{
		GoalID: "goal-feature", TodoID: "todo-impl", TurnID: "turn-1",
		Outcome:        controlplane.Outcome{Status: controlplane.OutcomeProgress},
		DecisionSource: "react-loop",
		PrimaryCause:   "tests green locally",
		Evidence: []controlplane.Evidence{
			{ID: "ev-1", Kind: "test_pass", Summary: "go test ./... green"},
		},
	})
	must(err)
	total, _ := k.SpendSlot(ctx, "goal-feature", deliveryID, controlplane.SpendOpts{Execute: true, Reason: "turn-1 delivered"})
	fmt.Printf("  writeback ok, delivery=%s, rolling-window spent=%d\n", deliveryID, total)

	fmt.Println("\n== 4. Open a concrete user gate -> ShouldRun blocks delivery ==")
	must(k.OpenGate(ctx, controlplane.UserGate{
		GateID: "gate-merge", GoalID: "goal-feature", TodoID: "todo-impl",
		Question: "Approve merge of PR #42 to main?",
		Scope:    controlplane.DecisionScope{Kind: controlplane.ScopePublicClaim, Granularity: controlplane.GranularityAction, ScopeKey: "refs/heads/main"},
	}))
	dec, _ = k.ShouldRun(ctx, "goal-feature", "agent-1")
	printDecision(dec)

	fmt.Println("\n== 5. Scoped fallback is authorized WITHOUT bypassing the gate ==")
	// Re-open with a fallback covering the same scope.
	must(k.ResolveGate(ctx, "goal-feature", "gate-merge", controlplane.GateOutcome{Decision: controlplane.DecisionCancel, By: "ops", Note: "reconsider"}))
	must(k.OpenGate(ctx, controlplane.UserGate{
		GateID: "gate-merge2", GoalID: "goal-feature", TodoID: "todo-impl",
		Question: "Approve merge of PR #42 to main?",
		Scope:    controlplane.DecisionScope{Kind: controlplane.ScopePublicClaim, ScopeKey: "refs/heads/main"},
		Fallback: &controlplane.FallbackPolicy{
			Scope:  controlplane.DecisionScope{Kind: controlplane.ScopePublicClaim, ScopeKey: "refs/heads/main"},
			Action: "open a draft PR instead of merging",
			Audit:  true, SpendOnce: true,
		},
	}))
	dec, _ = k.ShouldRun(ctx, "goal-feature", "agent-1")
	printDecision(dec)
	fmt.Printf("  -> fallback authorized? %v (gate still open)\n", dec.FallbackAuthorized)

	fmt.Println("\n== 6. Resolve gate -> delivery unblocked ==")
	must(k.ResolveGate(ctx, "goal-feature", "gate-merge2", controlplane.GateOutcome{Decision: controlplane.DecisionApprove, By: "alice"}))
	dec, _ = k.ShouldRun(ctx, "goal-feature", "agent-1")
	printDecision(dec)

	fmt.Println("\n== 7. Hard peer leases: CAS contention on the same todo ==")
	_, err = k.AcquireLease(ctx, "goal-feature", "todo-impl", "workerA", time.Minute)
	fmt.Printf("  workerA acquire: %v\n", errOr(err))
	_, err = k.AcquireLease(ctx, "goal-feature", "todo-impl", "workerB", time.Minute)
	fmt.Printf("  workerB acquire (expected ErrLeaseHeld): %v\n", errOr(err))
	must(todos.Upsert(ctx, &controlplane.Todo{ID: "todo-review", GoalID: "goal-feature", State: controlplane.TodoOpen, TaskClass: controlplane.TaskAdvancement}))
	_, err = k.AcquireLease(ctx, "goal-feature", "todo-review", "workerB", time.Minute)
	fmt.Printf("  workerB acquires DIFFERENT todo in parallel: %v\n", errOr(err))

	fmt.Println("\n== 8. Deterministic peer assignment (no durable leader) ==")
	for _, key := range []string{"replan:A", "replan:B", "replan:C"} {
		p, _ := controlplane.AssignPeer(key, []string{"agent-x", "agent-y", "agent-z"})
		fmt.Printf("  %s -> %s\n", key, p)
	}

	fmt.Println("\n== 9. Operator review packet (read-only projection) ==")
	pkt, err := k.ReviewPacket(ctx, "goal-feature")
	must(err)
	fmt.Printf("  objective: %s\n", pkt.Goal.Objective)
	fmt.Printf("  state: %s | open todos: %d | pending gates: %d\n", pkt.Goal.State, len(pkt.OpenTodos), len(pkt.PendingGates))
	fmt.Printf("  quota: spent=%d allowed=%d (compute=%.2f, window=%.1fh)\n", pkt.Quota.Spent, pkt.Quota.Allowed, pkt.Quota.Compute, pkt.Quota.WindowHours)
	fmt.Printf("  decision lineage (%d events):\n", len(pkt.DecisionLineage))
	for _, e := range pkt.DecisionLineage {
		fmt.Printf("    [%s] %s%s\n", e.Kind, e.Type, noteFor(e))
	}

	fmt.Println("\n== 10. Governance→evolution closed loop: completed goal → auto-solidify ==")
	// The gateway fires this automatically (AppConfig.Evolver +
	// AutoSolidifyOnGoalComplete); here we show the same payload on the Kernel
	// path. MockEvolver runs locally; swap for evolver.NewMCPEvolver(...) in
	// production to persist real Gene/Capsule records.
	ev := evolver.NewMockEvolver()
	goal.State = controlplane.GoalCompleted
	must(goals.Upsert(ctx, goal))
	sol, err := ev.Solidify(ctx, evolver.SolidifyRequest{
		Intent:         goal.Objective,
		Summary:        "control plane goal completed: " + goal.Objective,
		Signals:        []string{"goal_completed"},
		DecisionSource: "controlplane",
		PrimaryCause:   goal.ID,
	})
	must(err)
	fmt.Printf("  solidified: capsule=%s gene=%s\n", sol.CapsuleID, sol.GeneID)
	caps, err := ev.ListCapsules(ctx, 5)
	must(err)
	fmt.Printf("  evolver capsules now: %d (decision_source=controlplane)\n", len(caps))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func errOr(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

func noteFor(e controlplane.Event) string {
	if e.Outcome != "" {
		return " (" + e.Type + ":" + e.Outcome + ")"
	}
	return ""
}

func printDecision(d *controlplane.Decision) {
	fmt.Printf("  should_run=%v state=%s route=%s mode=%s notify=%s\n", d.ShouldRun, d.State, d.Route, d.Mode, d.Notify)
	if d.Question != "" {
		fmt.Printf("  question: %s\n", d.Question)
	}
	if d.Reason != "" {
		fmt.Printf("  reason: %s\n", d.Reason)
	}
}
