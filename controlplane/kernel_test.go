package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctxBG() context.Context { return context.Background() }

// seedKernel returns a Kernel with a fresh in-memory goal/todo/spend and a
// pre-registered active goal + open todo. The spend log is returned so tests
// can simulate prior spending directly.
func seedKernel(t *testing.T, q Quota) (*Kernel, *MemorySpendLog, *Goal, *Todo) {
	t.Helper()
	goals := NewMemoryGoalStore()
	todos := NewMemoryTodoStore()
	spend := NewMemorySpendLog()
	k := NewKernel(goals, todos, spend)

	g := &Goal{ID: "g1", Objective: "ship feature X", State: GoalActive, Quota: q}
	require.NoError(t, goals.Upsert(ctxBG(), g))
	todo := &Todo{ID: "t1", GoalID: "g1", Description: "do thing", TaskClass: TaskAdvancement, State: TodoOpen}
	require.NoError(t, todos.Upsert(ctxBG(), todo))
	g.CurrentTodoID = "t1"
	require.NoError(t, goals.Upsert(ctxBG(), g))
	return k, spend, g, todo
}

func TestLegalTransitions(t *testing.T) {
	// Goal
	assert.True(t, LegalGoalTransition(GoalActive, GoalPaused))
	assert.True(t, LegalGoalTransition(GoalPaused, GoalActive))
	assert.False(t, LegalGoalTransition(GoalCompleted, GoalActive)) // terminal
	assert.True(t, LegalGoalTransition(GoalActive, GoalActive))     // same-state trivially legal
	assert.True(t, GoalCompleted.IsTerminal())
	assert.False(t, GoalActive.IsTerminal())

	// Todo
	assert.True(t, LegalTodoTransition(TodoOpen, TodoDone))
	assert.True(t, LegalTodoTransition(TodoBlocked, TodoOpen))
	assert.False(t, LegalTodoTransition(TodoDone, TodoOpen)) // terminal
	assert.True(t, TodoDeferred.IsTerminal())
}

func TestTaskClassGating(t *testing.T) {
	assert.True(t, TaskUserGate.IsGating())
	assert.True(t, TaskBlocker.IsGating())
	assert.False(t, TaskMonitor.IsGating())
	assert.False(t, TaskUserAction.IsGating())
	assert.False(t, TaskAdvancement.IsGating())
}

func TestQuotaAllowedSlots(t *testing.T) {
	q := Quota{Compute: 0.5, WindowHours: 1.0, SlotMinutes: 15.0} // 60/15*0.5 = 2
	assert.Equal(t, 2, q.AllowedSlots())
	assert.Equal(t, 0, (Quota{Compute: 0, WindowHours: 1, SlotMinutes: 15}).AllowedSlots())
	assert.Equal(t, 0, (Quota{Compute: 1, WindowHours: 0, SlotMinutes: 15}).AllowedSlots())
}

func TestOutcomeAccountable(t *testing.T) {
	assert.True(t, OutcomeProgress.IsAccountable())
	assert.True(t, OutcomeCompletion.IsAccountable())
	assert.False(t, OutcomeFailed.IsAccountable())
}

func TestShouldRun(t *testing.T) {
	q := Quota{Compute: 0.5, WindowHours: 1.0, SlotMinutes: 15.0} // allowed=2

	tests := []struct {
		name      string
		setup     func(t *testing.T) (*Kernel, *MemorySpendLog)
		goalID    string
		wantRun   bool
		wantState ComputeState
		wantRoute TurnRoute
	}{
		{
			name: "goal not found -> contract error, no run",
			setup: func(t *testing.T) (*Kernel, *MemorySpendLog) {
				k, sp, _, _ := seedKernel(t, q)
				return k, sp
			},
			goalID:    "missing",
			wantRun:   false,
			wantState: ComputeBlockedHealth,
			wantRoute: RouteContractError,
		},
		{
			name: "terminal goal -> blocked, no run",
			setup: func(t *testing.T) (*Kernel, *MemorySpendLog) {
				k, sp, g, _ := seedKernel(t, q)
				g.State = GoalCompleted
				require.NoError(t, k.goals.Upsert(ctxBG(), g))
				return k, sp
			},
			goalID:    "g1",
			wantRun:   false,
			wantState: ComputeBlockedHealth,
			wantRoute: RouteBlocked,
		},
		{
			name: "paused goal -> terminal paused contract, no run",
			setup: func(t *testing.T) (*Kernel, *MemorySpendLog) {
				k, sp, g, _ := seedKernel(t, q)
				g.State = GoalPaused
				require.NoError(t, k.goals.Upsert(ctxBG(), g))
				return k, sp
			},
			goalID:    "g1",
			wantRun:   false,
			wantState: ComputePaused,
			wantRoute: RouteBlocked,
		},
		{
			name: "compute<=0 -> paused, no run",
			setup: func(t *testing.T) (*Kernel, *MemorySpendLog) {
				k, sp, _, _ := seedKernel(t, Quota{Compute: 0, WindowHours: 1, SlotMinutes: 15})
				return k, sp
			},
			goalID:    "g1",
			wantRun:   false,
			wantState: ComputePaused,
			wantRoute: RouteBlocked,
		},
		{
			name: "quota exhausted -> throttled, no run",
			setup: func(t *testing.T) (*Kernel, *MemorySpendLog) {
				k, sp, _, _ := seedKernel(t, q)
				// allowed=2; spend two prior slots in-window.
				_, err := sp.Append(ctxBG(), SpendEvent{GoalID: "g1", Slots: 1, SpentAt: time.Now()})
				require.NoError(t, err)
				_, err = sp.Append(ctxBG(), SpendEvent{GoalID: "g1", Slots: 1, SpentAt: time.Now()})
				require.NoError(t, err)
				return k, sp
			},
			goalID:    "g1",
			wantRun:   false,
			wantState: ComputeThrottled,
			wantRoute: RouteWait,
		},
		{
			name: "eligible -> run",
			setup: func(t *testing.T) (*Kernel, *MemorySpendLog) {
				k, sp, _, _ := seedKernel(t, q)
				return k, sp
			},
			goalID:    "g1",
			wantRun:   true,
			wantState: ComputeEligible,
			wantRoute: RouteReadyForHost,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, _ := tc.setup(t)
			dec, err := k.ShouldRun(ctxBG(), tc.goalID, "agent-1")
			require.NoError(t, err)
			require.NotNil(t, dec)
			assert.Equal(t, tc.wantRun, dec.ShouldRun)
			assert.Equal(t, tc.wantState, dec.State)
			assert.Equal(t, tc.wantRoute, dec.Route)
		})
	}
}

func TestClaimConflicts(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	// First claim succeeds.
	require.NoError(t, k.Claim(ctxBG(), "g1", "t1", "a1"))
	todo, err := k.todos.Get(ctxBG(), "g1", "t1")
	require.NoError(t, err)
	assert.Equal(t, "a1", todo.ClaimedBy)

	// Same agent re-claim is idempotent.
	require.NoError(t, k.Claim(ctxBG(), "g1", "t1", "a1"))

	// Different agent blocked.
	assert.ErrorIs(t, k.Claim(ctxBG(), "g1", "t1", "a2"), ErrAlreadyClaimed)

	// Non-open todo not claimable.
	require.NoError(t, k.todos.Upsert(ctxBG(), &Todo{ID: "t2", GoalID: "g1", State: TodoDone, TaskClass: TaskAdvancement}))
	assert.ErrorIs(t, k.Claim(ctxBG(), "g1", "t2", "a1"), ErrTodoNotClaimable)
}

func TestWritebackEvidenceInvariant(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	// Accountable outcome without evidence is rejected.
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-1",
		Outcome: Outcome{Status: OutcomeProgress},
		// no Evidence
	})
	assert.ErrorIs(t, err, ErrEvidenceRequired)

	// Failed outcome needs no evidence and records no delivery.
	deliveryID, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-fail",
		Outcome: Outcome{Status: OutcomeFailed},
	})
	require.NoError(t, err)
	assert.Empty(t, deliveryID)

	// Missing ids rejected.
	_, err = k.Writeback(ctxBG(), ValidatedWriteback{GoalID: "", TodoID: "t1"})
	assert.Error(t, err)
}

func TestWritebackCompletionTransitionsTodo(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-done",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e1", Kind: "test_pass", Summary: "tests green"}},
	})
	require.NoError(t, err)

	todo, err := k.todos.Get(ctxBG(), "g1", "t1")
	require.NoError(t, err)
	assert.Equal(t, TodoDone, todo.State)
	assert.Contains(t, todo.EvidenceIDs, "e1")
}

func TestSpendSlotRequiresValidatedWriteback(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	// No writeback -> rejected.
	_, err := k.SpendSlot(ctxBG(), "g1", "turn-phantom", SpendOpts{Execute: true})
	assert.ErrorIs(t, err, ErrNoValidatedWriteback)
}

func TestSpendSlotDryRunDoesNotCommit(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	deliveryID, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-dry",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)

	// Dry-run reports projected but does not commit.
	projected, err := k.SpendSlot(ctxBG(), "g1", deliveryID, SpendOpts{Execute: false})
	require.NoError(t, err)
	assert.Equal(t, 1, projected)

	// A second dry-run reports the same projected (still unspent).
	projected2, err := k.SpendSlot(ctxBG(), "g1", deliveryID, SpendOpts{Execute: false})
	require.NoError(t, err)
	assert.Equal(t, 1, projected2)
}

func TestSpendSlotIdempotent(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	deliveryID, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)

	// First execute commits.
	total, err := k.SpendSlot(ctxBG(), "g1", deliveryID, SpendOpts{Execute: true})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// Second execute on same turn -> already spent.
	_, err = k.SpendSlot(ctxBG(), "g1", deliveryID, SpendOpts{Execute: true})
	assert.ErrorIs(t, err, ErrAlreadySpent)
}

func TestSmokeTickEndToEnd(t *testing.T) {
	k, _, g, _ := seedKernel(t, DefaultQuota())

	// 1. Tick: ShouldRun=true + claims current todo.
	res, err := k.Tick(ctxBG(), g.ID, "agent-1", "turn-1")
	require.NoError(t, err)
	require.NotNil(t, res.Decision)
	assert.True(t, res.Decision.ShouldRun)
	assert.Equal(t, "t1", res.ClaimedTodoID)

	// 2. Runtime executes work (out of band), then writeback with evidence.
	deliveryID, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: g.ID, TodoID: "t1", TurnID: "turn-1",
		Outcome:        Outcome{Status: OutcomeCompletion},
		DecisionSource: "react-loop",
		PrimaryCause:   "tests green",
		Evidence:       []Evidence{{ID: "e1", Kind: "test_pass", Summary: "all pass"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "turn-1", deliveryID)

	// 3. Spend one slot.
	total, err := k.SpendSlot(ctxBG(), g.ID, deliveryID, SpendOpts{Execute: true})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// 4. Second spend on same turn rejected (idempotent).
	_, err = k.SpendSlot(ctxBG(), g.ID, deliveryID, SpendOpts{Execute: true})
	assert.ErrorIs(t, err, ErrAlreadySpent)

	// 5. Todo is done (terminal).
	todo, err := k.todos.Get(ctxBG(), g.ID, "t1")
	require.NoError(t, err)
	assert.Equal(t, TodoDone, todo.State)
}

func TestGoalPersistenceAcrossRestart(t *testing.T) {
	// P0 in-memory store simulates "restart" by re-reading from the same store.
	store := NewMemoryGoalStore()
	require.NoError(t, store.Upsert(ctxBG(), &Goal{ID: "g-restart", Objective: "persist", State: GoalActive, Quota: DefaultQuota()}))

	g, err := store.Get(ctxBG(), "g-restart")
	require.NoError(t, err)
	assert.Equal(t, "persist", g.Objective)
	assert.False(t, g.CreatedAt.IsZero())

	// Terminal transition persisted.
	require.True(t, LegalGoalTransition(g.State, GoalCompleted))
	g.State = GoalCompleted
	require.NoError(t, store.Upsert(ctxBG(), g))

	g2, err := store.Get(ctxBG(), "g-restart")
	require.NoError(t, err)
	assert.Equal(t, GoalCompleted, g2.State)
}

func TestWritebackIdempotentSameTurn(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	wb := ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-x",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	}
	id1, err := k.Writeback(ctxBG(), wb)
	require.NoError(t, err)
	// Same turn writeback again is a no-op (returns same id, no duplicate delivery).
	id2, err := k.Writeback(ctxBG(), wb)
	require.NoError(t, err)
	assert.Equal(t, id1, id2)

	// Only one spend possible.
	_, err = k.SpendSlot(ctxBG(), "g1", id1, SpendOpts{Execute: true})
	require.NoError(t, err)
	_, err = k.SpendSlot(ctxBG(), "g1", id1, SpendOpts{Execute: true})
	assert.ErrorIs(t, err, ErrAlreadySpent)
}

// Ensure error sentinels satisfy errors.Is via errors.Is (not just ==).
func TestErrorSentinels(t *testing.T) {
	assert.True(t, errors.Is(ErrAlreadyClaimed, ErrAlreadyClaimed))
	assert.True(t, errors.Is(ErrEvidenceRequired, ErrEvidenceRequired))
}

// --- P1: gates, fallback, ledger ---

func TestDecisionScopeCovers(t *testing.T) {
	g := DecisionScope{Kind: ScopeWrite, Granularity: GranularityAction, ScopeKey: "src/foo.go"}
	assert.True(t, g.Covers(DecisionScope{Kind: ScopeWrite, ScopeKey: "src/foo.go"}))
	assert.False(t, g.Covers(DecisionScope{Kind: ScopeWrite, ScopeKey: "src/other.go"}))
	assert.False(t, g.Covers(DecisionScope{Kind: ScopeProduction, ScopeKey: "src/foo.go"}))
}

func openTestGate(t *testing.T, k *Kernel, withFallback *FallbackPolicy) string {
	t.Helper()
	gate := UserGate{
		GateID: "gate-1", GoalID: "g1", TodoID: "t1",
		Question: "May I push to main?",
		Scope:    DecisionScope{Kind: ScopePublicClaim, Granularity: GranularityAction, ScopeKey: "refs/heads/main"},
		Fallback: withFallback,
	}
	require.NoError(t, k.OpenGate(ctxBG(), gate))
	return gate.GateID
}

func TestShouldRunGateBlocksDelivery(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	openTestGate(t, k, nil)

	dec, err := k.ShouldRun(ctxBG(), "g1", "a1")
	require.NoError(t, err)
	assert.False(t, dec.ShouldRun)
	assert.Equal(t, ComputeOperatorGate, dec.State)
	assert.Equal(t, RouteUserActionRequired, dec.Route)
	assert.Equal(t, ModeUserGate, dec.Mode)
	assert.Equal(t, Notify, dec.Notify) // gate surfaces a concrete question
	assert.Equal(t, "gate-1", dec.GateID)
	assert.Contains(t, dec.Question, "push to main")
	assert.False(t, dec.FallbackAuthorized)

	// Tick must NOT claim while the gate is open.
	res, err := k.Tick(ctxBG(), "g1", "a1", "turn-1")
	require.NoError(t, err)
	assert.Empty(t, res.ClaimedTodoID)
}

func TestResolveGateUnblocksDelivery(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	gateID := openTestGate(t, k, nil)

	// Blocked while open.
	dec, _ := k.ShouldRun(ctxBG(), "g1", "a1")
	assert.False(t, dec.ShouldRun)

	// Approve -> eligible.
	require.NoError(t, k.ResolveGate(ctxBG(), "g1", gateID, GateOutcome{
		Decision: DecisionApprove, By: "alice", Note: "lgtm",
	}))
	dec2, err := k.ShouldRun(ctxBG(), "g1", "a1")
	require.NoError(t, err)
	assert.True(t, dec2.ShouldRun)
	assert.Equal(t, ComputeEligible, dec2.State)

	// Resolving again is rejected (idempotent).
	err = k.ResolveGate(ctxBG(), "g1", gateID, GateOutcome{Decision: DecisionApprove})
	assert.ErrorIs(t, err, ErrGateAlreadyResolved)
}

func TestFallbackAuthorizedButDoesNotBypassGate(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	fb := &FallbackPolicy{
		Scope:  DecisionScope{Kind: ScopePublicClaim, ScopeKey: "refs/heads/main"},
		Action: "open a draft PR instead of pushing",
		Audit:  true, SpendOnce: true,
	}
	openTestGate(t, k, fb)

	// ShouldRun reports the gate with an authorized scoped fallback. The gated
	// lane itself stays blocked (should_run=false); only the fallback may run.
	dec, err := k.ShouldRun(ctxBG(), "g1", "a1")
	require.NoError(t, err)
	assert.False(t, dec.ShouldRun)
	assert.Equal(t, ModeScopedUserGateFallback, dec.Mode)
	assert.True(t, dec.FallbackAuthorized)
	require.NotNil(t, dec.Fallback)
	assert.Equal(t, "open a draft PR instead of pushing", dec.Fallback.Action)

	// Runtime executes the fallback and writebacks with evidence, then spends.
	deliveryID, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-fb",
		Outcome:  Outcome{Status: OutcomeProgress, Score: 0.5},
		Evidence: []Evidence{{ID: "ef", Kind: "pr_opened", Summary: "draft #42"}},
	})
	require.NoError(t, err)
	total, err := k.SpendSlot(ctxBG(), "g1", deliveryID, SpendOpts{Execute: true})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// The gate is STILL unresolved: the fallback did not bypass it. Next
	// ShouldRun keeps reporting the gate.
	dec2, _ := k.ShouldRun(ctxBG(), "g1", "a1")
	assert.False(t, dec2.ShouldRun)
	assert.Equal(t, ComputeOperatorGate, dec2.State)
}

func TestOpenGateRejectsFallbackScopeMismatch(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	// Gate covers main; fallback claims a different scope -> must be rejected.
	gate := UserGate{
		GateID: "gx", GoalID: "g1",
		Scope: DecisionScope{Kind: ScopePublicClaim, ScopeKey: "refs/heads/main"},
		Fallback: &FallbackPolicy{
			Scope:  DecisionScope{Kind: ScopeProduction, ScopeKey: "deploy"}, // different
			Action: "deploy",
		},
	}
	err := k.OpenGate(ctxBG(), UserGate(gate))
	assert.ErrorIs(t, err, ErrFallbackScopeMismatch)
}

func TestResolveGateRejectsUnauthorizedResolver(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	// Gate names two authorized resolvers.
	require.NoError(t, k.OpenGate(ctxBG(), UserGate{
		GateID: "gR", GoalID: "g1", Question: "merge?",
		Scope:     DecisionScope{Kind: ScopePublicClaim, ScopeKey: "main"},
		Resolvers: []string{"alice", "bob"},
	}))
	// Eve is not in the resolver list -> rejected (#5).
	err := k.ResolveGate(ctxBG(), "g1", "gR", GateOutcome{Decision: DecisionApprove, By: "eve"})
	assert.ErrorIs(t, err, ErrUnauthorizedResolver)

	// Alice is authorized -> resolves.
	require.NoError(t, k.ResolveGate(ctxBG(), "g1", "gR", GateOutcome{Decision: DecisionApprove, By: "alice"}))
	g, _ := k.GateStore().Get(ctxBG(), "g1", "gR")
	assert.True(t, g.IsResolved())
}

func TestLedgerRecordsDecisionLineage(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	gateID := openTestGate(t, k, nil)

	// Resolve gate, then run a validated writeback + spend.
	require.NoError(t, k.ResolveGate(ctxBG(), "g1", gateID, GateOutcome{Decision: DecisionApprove}))
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-1",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e1", Kind: "test_pass", Summary: "green"}},
	})
	require.NoError(t, err)
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-1", SpendOpts{Execute: true})
	require.NoError(t, err)

	// Ledger must contain the lineage in order: gate_opened, gate_resolved,
	// validated_writeback, quota_slot_spent.
	led := k.Ledger()
	require.NotNil(t, led)
	evs, _, err := led.Read(ctxBG(), "g1", 0, 0)
	require.NoError(t, err)
	require.Len(t, evs, 4)
	assert.Equal(t, "gate_opened", evs[0].Type)
	assert.Equal(t, EventDecision, evs[0].Kind)
	assert.Equal(t, "gate_resolved", evs[1].Type)
	assert.Equal(t, "approve", evs[1].Outcome)
	assert.Equal(t, "validated_writeback", evs[2].Type)
	assert.Equal(t, EventWork, evs[2].Kind)
	assert.Equal(t, "completion", evs[2].Outcome)
	assert.Equal(t, "quota_slot_spent", evs[3].Type)
	assert.Equal(t, EventAccounting, evs[3].Kind)
	// Monotonic per-goal indices.
	for i, e := range evs {
		assert.Equal(t, int64(i), e.Index)
	}
}

// --- P2: leases, peer assignment, review board ---

func TestLeaseSameTodoCASConflict(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	// Worker A acquires the lease on t1.
	la, err := k.AcquireLease(ctxBG(), "g1", "t1", "workerA", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "workerA", la.Owner)
	assert.True(t, la.IsValid(time.Now()))

	// Worker B is blocked on the same todo (CAS conflict).
	_, err = k.AcquireLease(ctxBG(), "g1", "t1", "workerB", time.Minute)
	assert.ErrorIs(t, err, ErrLeaseHeld)

	// Worker A re-acquiring (same owner) is idempotent / renew.
	la2, err := k.AcquireLease(ctxBG(), "g1", "t1", "workerA", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "workerA", la2.Owner)

	// A different todo under the same goal is independent -> parallel OK.
	require.NoError(t, k.todos.Upsert(ctxBG(), &Todo{ID: "t2", GoalID: "g1", State: TodoOpen, TaskClass: TaskAdvancement}))
	lb, err := k.AcquireLease(ctxBG(), "g1", "t2", "workerB", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "workerB", lb.Owner)
}

func TestLeaseOwnerGuard(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	_, err := k.AcquireLease(ctxBG(), "g1", "t1", "owner1", time.Minute)
	require.NoError(t, err)

	// Renew/Release/Transfer by a non-owner must fail.
	_, err = k.leases.Renew(ctxBG(), "g1", "t1", "intruder", time.Minute)
	assert.ErrorIs(t, err, ErrLeaseOwnerMismatch)
	err = k.ReleaseLease(ctxBG(), "g1", "t1", "intruder")
	assert.ErrorIs(t, err, ErrLeaseOwnerMismatch)

	// Transfer hands off; new owner can release.
	_, err = k.leases.Transfer(ctxBG(), "g1", "t1", "owner1", "owner2")
	require.NoError(t, err)
	require.NoError(t, k.ReleaseLease(ctxBG(), "g1", "t1", "owner2"))
	_, err = k.InspectLease(ctxBG(), "g1", "t1")
	assert.ErrorIs(t, err, ErrLeaseNotFound)
}

func TestLeaseTTLExpiry(t *testing.T) {
	store := NewMemoryLeaseStore()
	// Acquire with a tiny TTL, then wait past expiry and confirm Inspect reaps it.
	l, err := store.Acquire(ctxBG(), "g", "t", "o", 30*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, l.IsValid(time.Now()))
	time.Sleep(60 * time.Millisecond)
	got, err := store.Inspect(ctxBG(), "g", "t")
	assert.ErrorIs(t, err, ErrLeaseNotFound)
	assert.Nil(t, got)
}

func TestAssignPeerStableAndOrderInvariant(t *testing.T) {
	agents := []string{"c", "a", "b", "a"} // unsorted, with duplicate
	key := "replan:goal-1:unscoped-obligation"

	// Same key+set always picks the same peer.
	p1, err := AssignPeer(key, agents)
	require.NoError(t, err)
	p2, err := AssignPeer(key, []string{"a", "b", "c"}) // different order, same set
	require.NoError(t, err)
	assert.Equal(t, p1, p2, "assignment must be invariant to registration order")

	// Different key on the same set distributes (sanity: not always same).
	seen := map[string]bool{}
	for _, k := range []string{"k1", "k2", "k3", "k4", "k5", "k6"} {
		p, err := AssignPeer(k, []string{"a", "b", "c"})
		require.NoError(t, err)
		seen[p] = true
	}
	assert.Greater(t, len(seen), 1, "different keys should distribute across peers")

	// Empty set errors.
	_, err = AssignPeer(key, nil)
	assert.Error(t, err)
}

func TestReviewPacketAggregation(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())

	// Open a gate, resolve it, run a writeback+spend, and open another gate.
	gateID := openTestGate(t, k, nil)
	require.NoError(t, k.ResolveGate(ctxBG(), "g1", gateID, GateOutcome{Decision: DecisionApprove}))
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-r1",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e1", Kind: "test_pass", Summary: "green"}},
	})
	require.NoError(t, err)
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-r1", SpendOpts{Execute: true})
	require.NoError(t, err)
	// A second, still-open gate under a fresh id.
	require.NoError(t, k.OpenGate(ctxBG(), UserGate{
		GateID: "gate-2", GoalID: "g1", TodoID: "t1",
		Question: "ship now?", Scope: DecisionScope{Kind: ScopeDirection, ScopeKey: "release"},
	}))

	pkt, err := k.ReviewPacket(ctxBG(), "g1")
	require.NoError(t, err)
	require.NotNil(t, pkt)
	assert.Equal(t, "g1", pkt.Goal.ID)
	// t1 was completed by the writeback, so it should NOT appear in OpenTodos.
	for _, ot := range pkt.OpenTodos {
		assert.NotEqual(t, "t1", ot.ID)
	}
	// One pending gate remains (gate-2).
	require.Len(t, pkt.PendingGates, 1)
	assert.Equal(t, "gate-2", pkt.PendingGates[0].GateID)
	// Recent work shows the validated writeback.
	require.NotEmpty(t, pkt.RecentWork)
	assert.Equal(t, "validated_writeback", pkt.RecentWork[len(pkt.RecentWork)-1].Type)
	// Decision lineage has the full sequence.
	assert.GreaterOrEqual(t, len(pkt.DecisionLineage), 4)
	// Quota reflects one spent slot.
	assert.Equal(t, 1, pkt.Quota.Spent)
	assert.Equal(t, DefaultQuota().AllowedSlots(), pkt.Quota.Allowed)
}

func TestReviewPacketMissingGoal(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	_, err := k.ReviewPacket(ctxBG(), "nope")
	assert.ErrorIs(t, err, ErrGoalNotFound)
}

// --- P2-8: Kanban projection + row lineage ---

func TestSupersedeTodoLinksLineage(t *testing.T) {
	k, _, _, todo := seedKernel(t, DefaultQuota())
	original := todo.ID
	// Claim the todo first so we can validate owner inheritance on the successor.
	require.NoError(t, k.Claim(ctxBG(), "g1", original, "a1"))

	succ, err := k.SupersedeTodo(ctxBG(), "g1", original, "a1", "revised approach: add tests first", "old approach flaky")
	require.NoError(t, err)
	assert.NotEqual(t, original, succ.ID)
	assert.Equal(t, original, succ.Supersedes, "successor must record its predecessor")
	assert.Equal(t, "a1", succ.ClaimedBy, "successor inherits owner for smooth handoff")

	// Old todo is deferred + linked to successor.
	old, err := k.todos.Get(ctxBG(), "g1", original)
	require.NoError(t, err)
	assert.Equal(t, TodoDeferred, old.State)
	assert.Equal(t, succ.ID, old.SupersededBy)

	// Double-supersede is rejected.
	_, err = k.SupersedeTodo(ctxBG(), "g1", original, "a1", "again", "")
	assert.Error(t, err)

	// Ledger recorded the row_lifecycle event.
	led := k.Ledger()
	evs, _, _ := led.Read(ctxBG(), "g1", 0, 0)
	found := false
	for _, e := range evs {
		if e.Type == "todo_superseded" {
			assert.Equal(t, string(RowSuperseded), e.Detail["row_lifecycle"])
			assert.Equal(t, original, e.Detail["superseded"])
			found = true
		}
	}
	assert.True(t, found, "ledger must record todo_superseded event")
}

func TestSupersedeAdvancesCurrentTodo(t *testing.T) {
	k, _, g, todo := seedKernel(t, DefaultQuota())
	require.Equal(t, todo.ID, g.CurrentTodoID)

	succ, err := k.SupersedeTodo(ctxBG(), "g1", todo.ID, "a1", "new approach", "")
	require.NoError(t, err)

	g2, _ := k.goals.Get(ctxBG(), "g1")
	assert.Equal(t, succ.ID, g2.CurrentTodoID, "goal's current todo advances to successor")
}

func TestKanbanProjectionAndLineage(t *testing.T) {
	k, _, _, todo := seedKernel(t, DefaultQuota())

	// Supersede t1 -> t2, and add an independent open todo t3 + a done todo t4.
	succ, err := k.SupersedeTodo(ctxBG(), "g1", todo.ID, "a1", "v2", "")
	require.NoError(t, err)
	require.NoError(t, k.todos.Upsert(ctxBG(), &Todo{ID: "t3", GoalID: "g1", State: TodoOpen, TaskClass: TaskAdvancement, Description: "side task"}))
	require.NoError(t, k.todos.Upsert(ctxBG(), &Todo{ID: "t4", GoalID: "g1", State: TodoDone, TaskClass: TaskAdvancement, Description: "done task"}))

	board, err := k.Kanban(ctxBG(), "g1")
	require.NoError(t, err)
	assert.Equal(t, "g1", board.GoalID)

	// Deferred column carries the superseded t1 with RowSuperseded lifecycle.
	deferred := board.Columns[TodoDeferred]
	require.Len(t, deferred, 1)
	assert.Equal(t, todo.ID, deferred[0].Todo.ID)
	assert.Equal(t, RowSuperseded, deferred[0].Lifecycle)
	assert.Equal(t, succ.ID, deferred[0].SupersededBy)

	// Open column has the successor + t3, both RowCurrent.
	openCards := board.Columns[TodoOpen]
	assert.GreaterOrEqual(t, len(openCards), 2)
	for _, c := range openCards {
		assert.Equal(t, RowCurrent, c.Lifecycle)
	}

	// Done column: t4 RowCurrent.
	require.Contains(t, board.Columns, TodoDone)

	// Lineage edge recorded.
	require.Len(t, board.Lineage, 1)
	assert.Equal(t, todo.ID, board.Lineage[0].From)
	assert.Equal(t, succ.ID, board.Lineage[0].To)
}

func TestKanbanFoldsGatesOntoCards(t *testing.T) {
	k, _, _, todo := seedKernel(t, DefaultQuota())
	// Open a gate bound to the todo.
	require.NoError(t, k.OpenGate(ctxBG(), UserGate{
		GateID: "gg", GoalID: "g1", TodoID: todo.ID, Question: "ok?",
		Scope: DecisionScope{Kind: ScopeWrite, ScopeKey: "x"},
	}))
	board, err := k.Kanban(ctxBG(), "g1")
	require.NoError(t, err)
	for _, c := range board.Columns[TodoOpen] {
		if c.Todo.ID == todo.ID {
			assert.True(t, c.HasOpenGate, "gate must fold onto the matching card")
			return
		}
	}
	assert.Fail(t, "todo card not found in open column")
}

func TestKanbanMissingGoal(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	_, err := k.Kanban(ctxBG(), "ghost")
	assert.ErrorIs(t, err, ErrGoalNotFound)
}

// --- #3: ticket enforcement + claim ownership ---

func TestReapNeverRemovesUnconsumedTicket(t *testing.T) {
	// long-running turn may authorize (ShouldRunTurn) and not spend until much
	// later; its authorization must survive a background sweep.
	ts := NewMemoryTicketStore()
	ctx := ctxBG()
	tok, err := ts.Mint(ctx, "g", "turn-long")
	require.NoError(t, err)

	// Reap with an aggressive cutoff — the unconsumed ticket must remain.
	require.NoError(t, ts.Reap(ctx, 24*time.Hour))
	require.NoError(t, ts.Consume(ctx, "g", "turn-long", tok),
		"unconsumed ticket must survive reap and still be consumable")
}

func TestTicketEnforcementBlocksSpendWithoutShouldRun(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	k.WithTicketEnforcement()

	// Writeback creates a delivery, but no ShouldRunTurn was called -> no ticket.
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-1", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)

	_, err = k.SpendSlot(ctxBG(), "g1", "turn-1", SpendOpts{Execute: true, Token: "bogus"})
	assert.ErrorIs(t, err, ErrNoShouldRunTicket, "spend without ShouldRun must be rejected when enforcement is on")
}

func TestTicketFlowAuthorizeThenSpendThenConsumed(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	k.WithTicketEnforcement()

	// Authorize the turn first; capture the minted token.
	dec, err := k.ShouldRunTurn(ctxBG(), "g1", "a1", "turn-1")
	require.NoError(t, err)
	assert.True(t, dec.ShouldRun)
	assert.NotEmpty(t, dec.TurnToken, "eligible ShouldRunTurn mints a token")

	// Writeback + spend with the presented token now succeed.
	_, err = k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-1", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-1", SpendOpts{Execute: true, Token: dec.TurnToken})
	require.NoError(t, err)

	// Ticket is consumed; a second spend on the same turn is rejected.
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-1", SpendOpts{Execute: true, Token: dec.TurnToken})
	assert.Error(t, err)
}

func TestTicketRejectsStaleAuthorizationAfterGateOpens(t *testing.T) {
	// Round-3 #3: a ticket is a SNAPSHOT. If a gate opens between ShouldRunTurn
	// and SpendSlot, the spend must be rejected (the turn is no longer eligible).
	k, _, _, _ := seedKernel(t, DefaultQuota())
	k.WithTicketEnforcement()

	dec, err := k.ShouldRunTurn(ctxBG(), "g1", "a1", "turn-stale")
	require.NoError(t, err)
	require.True(t, dec.ShouldRun)
	require.NotEmpty(t, dec.TurnToken)

	_, err = k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-stale", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)

	// A gate opens AFTER the work was authorized.
	require.NoError(t, k.OpenGate(ctxBG(), UserGate{
		GateID: "gL", GoalID: "g1", Question: "new review required",
		Scope: DecisionScope{Kind: ScopeWrite, ScopeKey: "x"},
	}))

	_, err = k.SpendSlot(ctxBG(), "g1", "turn-stale", SpendOpts{Execute: true, Token: dec.TurnToken})
	assert.ErrorIs(t, err, ErrStaleTicket, "spend must be rejected when a gate opened mid-turn")

	// Resolving the gate makes the turn eligible again; spend then succeeds.
	require.NoError(t, k.ResolveGate(ctxBG(), "g1", "gL", GateOutcome{Decision: DecisionApprove, By: "a1"}))
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-stale", SpendOpts{Execute: true, Token: dec.TurnToken})
	require.NoError(t, err)
}

func TestTicketRejectsWrongOrMissingToken(t *testing.T) {
	k, _, _, _ := seedKernel(t, DefaultQuota())
	k.WithTicketEnforcement()

	dec, err := k.ShouldRunTurn(ctxBG(), "g1", "a1", "turn-2")
	require.NoError(t, err)
	require.NotEmpty(t, dec.TurnToken)

	_, err = k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-2", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)

	// Missing token -> rejected (#3: token required when enforcement on).
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-2", SpendOpts{Execute: true})
	assert.ErrorIs(t, err, ErrTicketTokenMismatch)

	// Wrong token -> rejected.
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-2", SpendOpts{Execute: true, Token: "bogus"})
	assert.Error(t, err)

	// Correct token -> ok.
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-2", SpendOpts{Execute: true, Token: dec.TurnToken})
	require.NoError(t, err)
}

func TestTicketEnforcementOffByDefault(t *testing.T) {
	// Without WithTicketEnforcement, spend works without ShouldRun (advisory).
	k, _, _, _ := seedKernel(t, DefaultQuota())
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: "t1", TurnID: "turn-1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)
	_, err = k.SpendSlot(ctxBG(), "g1", "turn-1", SpendOpts{Execute: true})
	require.NoError(t, err, "default (advisory) allows spend without ticket")
}

func TestWritebackClaimOwnership(t *testing.T) {
	k, _, _, todo := seedKernel(t, DefaultQuota())
	require.NoError(t, k.Claim(ctxBG(), "g1", todo.ID, "a1"))

	// a2 cannot write back a1's claimed todo.
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: todo.ID, TurnID: "t-x", AgentID: "a2",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e", Kind: "k", Summary: "s"}},
	})
	assert.ErrorIs(t, err, ErrClaimOwnerMismatch)

	// a1 can.
	_, err = k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: "g1", TodoID: todo.ID, TurnID: "t-x", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e", Kind: "k", Summary: "s"}},
	})
	require.NoError(t, err)
}

// --- #4: store slice isolation ---

func TestStoreSliceIsolation(t *testing.T) {
	gs := NewMemoryGoalStore()
	require.NoError(t, gs.Upsert(ctxBG(), &Goal{
		ID: "g", Objective: "o", State: GoalActive, Quota: DefaultQuota(),
		Scope: []string{"a", "b"},
	}))
	g, err := gs.Get(ctxBG(), "g")
	require.NoError(t, err)
	// Mutate the returned slice; the store's copy must be unaffected.
	g.Scope[0] = "MUTATED"
	g2, _ := gs.Get(ctxBG(), "g")
	assert.Equal(t, "a", g2.Scope[0], "store slice isolated from caller mutation (#4)")

	// Todo evidence ids likewise.
	ts := NewMemoryTodoStore()
	require.NoError(t, ts.Upsert(ctxBG(), &Todo{ID: "t", GoalID: "g", State: TodoOpen, EvidenceIDs: []string{"e1"}}))
	td, _ := ts.Get(ctxBG(), "g", "t")
	td.EvidenceIDs[0] = "hack"
	td2, _ := ts.Get(ctxBG(), "g", "t")
	assert.Equal(t, "e1", td2.EvidenceIDs[0], "todo evidence slice isolated (#4)")
}

func TestGateStoreSliceIsolation(t *testing.T) {
	s := NewMemoryGateStore()
	require.NoError(t, s.Upsert(ctxBG(), UserGate{
		GateID: "gg", GoalID: "g", Question: "q?", Resolvers: []string{"a", "b"},
	}))
	g, _ := s.Get(ctxBG(), "g", "gg")
	g.Resolvers[0] = "intruder"
	g2, _ := s.Get(ctxBG(), "g", "gg")
	assert.Equal(t, "a", g2.Resolvers[0], "gate resolvers slice isolated (#4)")
}

// --- #3b round-4: reward memory integration ---

func TestRewardHardPolicyVetoesShouldRun(t *testing.T) {
	k, _, g, _ := seedKernel(t, DefaultQuota())
	// Record an active hard_policy veto for the goal.
	require.NoError(t, k.RecordReward(ctxBG(), g.ID, RewardRecord{
		Class: AuthorityHardPolicy, Lifecycle: LifecycleActive,
		Scope:   DecisionScope{Kind: ScopeDirection, Granularity: GranularityGoal, ScopeKey: g.ID},
		Content: "no deploys until audit clears",
	}))
	dec, err := k.ShouldRun(ctxBG(), g.ID, "a1")
	require.NoError(t, err)
	assert.False(t, dec.ShouldRun)
	assert.Equal(t, ComputePolicyBlocked, dec.State)
	assert.Equal(t, ModePolicyBlocked, dec.Mode)
	assert.Contains(t, dec.Reason, "no deploys until audit clears")
}

func TestWritebackAutoRecordsRunBoundReward(t *testing.T) {
	k, _, g, todo := seedKernel(t, DefaultQuota())
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: g.ID, TodoID: todo.ID, TurnID: "t1", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)
	recs, err := k.RewardStore().List(ctxBG(), g.ID)
	require.NoError(t, err)
	require.NotEmpty(t, recs)
	assert.Equal(t, AuthorityRunBoundReward, recs[0].Class)
}

// --- #3c round-4: capability lane gate enforcement ---

func TestLaneGateBlocksCompletionWithoutApprovedGate(t *testing.T) {
	k, _, g, _ := seedKernel(t, DefaultQuota())
	g.CapabilityID = "issue-fix"
	require.NoError(t, k.GoalStore().Upsert(ctxBG(), g))
	// A todo at the gated "merge" stage of the issue-fix lane.
	mergeTodo := &Todo{ID: "tm", GoalID: g.ID, StageID: "merge", State: TodoOpen, TaskClass: TaskAdvancement}
	require.NoError(t, k.TodoStore().Upsert(ctxBG(), mergeTodo))

	// Completing merge WITHOUT an approved gate -> ErrLaneGateRequired.
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: g.ID, TodoID: "tm", TurnID: "t1",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e1", Kind: "merge", Summary: "merged"}},
	})
	assert.ErrorIs(t, err, ErrLaneGateRequired)

	// An OPEN gate on the todo still blocks (not approved yet).
	require.NoError(t, k.OpenGate(ctxBG(), UserGate{GateID: "gm", GoalID: g.ID, TodoID: "tm", Question: "approve merge?", Scope: DecisionScope{Kind: ScopePublicClaim, ScopeKey: "main"}}))
	_, err = k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: g.ID, TodoID: "tm", TurnID: "t2",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e2", Kind: "merge", Summary: "merged"}},
	})
	assert.ErrorIs(t, err, ErrLaneGateRequired, "open gate still blocks completion")

	// Resolve approve -> completion now allowed.
	require.NoError(t, k.ResolveGate(ctxBG(), g.ID, "gm", GateOutcome{Decision: DecisionApprove, By: "lead"}))
	_, err = k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: g.ID, TodoID: "tm", TurnID: "t3",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e3", Kind: "merge", Summary: "merged"}},
	})
	require.NoError(t, err)
	td, _ := k.todos.Get(ctxBG(), g.ID, "tm")
	assert.Equal(t, TodoDone, td.State)
}

func TestLaneGateIgnoredForNonGatedStage(t *testing.T) {
	k, _, g, _ := seedKernel(t, DefaultQuota())
	g.CapabilityID = "issue-fix"
	require.NoError(t, k.GoalStore().Upsert(ctxBG(), g))
	// "patch" is NOT a gated stage -> completion needs no gate.
	patchTodo := &Todo{ID: "tp", GoalID: g.ID, StageID: "patch", State: TodoOpen, TaskClass: TaskAdvancement}
	require.NoError(t, k.TodoStore().Upsert(ctxBG(), patchTodo))
	_, err := k.Writeback(ctxBG(), ValidatedWriteback{
		GoalID: g.ID, TodoID: "tp", TurnID: "t1",
		Outcome:  Outcome{Status: OutcomeCompletion},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "patched"}},
	})
	require.NoError(t, err, "non-gated stage completes without a gate")
}
