package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite" // register the sqlite driver for tests

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func newSQLKernelInMem(t *testing.T) *Kernel {
	t.Helper()
	k, err := NewSQLKernel(openDB(t, ":memory:"))
	require.NoError(t, err)
	return k
}

func TestSQLStoresDurabilityAcrossRestart(t *testing.T) {
	// Use a file DB so we can close + reopen and prove state survives (#1).
	path := filepath.Join(t.TempDir(), "cp.sqlite")

	// Write phase.
	db := openDB(t, path)
	k, err := NewSQLKernel(db)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, k.GoalStore().Upsert(ctx, &Goal{
		ID: "g-restart", OwnerUserID: "alice", Objective: "ship safely",
		State: GoalActive, Quota: DefaultQuota(),
	}))
	require.NoError(t, k.TodoStore().Upsert(ctx, &Todo{
		ID: "t1", GoalID: "g-restart", OwnerUserID: "alice",
		Description: "implement", TaskClass: TaskAdvancement, State: TodoOpen,
		Evidence: []Evidence{{ID: "e1"}, {ID: "e2"}},
	}))
	g, _ := k.GoalStore().Get(ctx, "g-restart")
	g.CurrentTodoID = "t1"
	require.NoError(t, k.GoalStore().Upsert(ctx, g))
	db.Close()

	// Reopen on a fresh connection — data must persist.
	db2 := openDB(t, path)
	k2, err := NewSQLKernel(db2)
	require.NoError(t, err)

	g2, err := k2.GoalStore().Get(ctx, "g-restart")
	require.NoError(t, err)
	assert.Equal(t, "ship safely", g2.Objective)
	assert.Equal(t, "alice", g2.OwnerUserID)
	assert.Equal(t, GoalActive, g2.State)
	assert.Equal(t, "t1", g2.CurrentTodoID)
	assert.Equal(t, 0.5, g2.Quota.Compute)

	todo, err := k2.TodoStore().Get(ctx, "g-restart", "t1")
	require.NoError(t, err)
	assert.Equal(t, "implement", todo.Description)
	assert.Equal(t, []string{"e1", "e2"}, todo.EvidenceIDs(), "derived evidence ids survived restart (#5)")
}

func TestSQLCrossProcessWritebackSpend(t *testing.T) {
	// #4 round-4: deliveries are shared like tickets. Kernel A writes back,
	// Kernel B spends — simulating two processes sharing one DB.
	db := openDB(t, ":memory:")
	kA, err := NewSQLKernel(db)
	require.NoError(t, err)
	kB, err := NewSQLKernel(db) // separate Kernel instance, same DB
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, kA.GoalStore().Upsert(ctx, &Goal{ID: "g", Objective: "o", State: GoalActive, Quota: DefaultQuota()}))
	require.NoError(t, kA.TodoStore().Upsert(ctx, &Todo{ID: "t1", GoalID: "g", State: TodoOpen, TaskClass: TaskAdvancement}))

	// Process A: write back (creates a delivery in the SHARED store).
	_, err = kA.Writeback(ctx, ValidatedWriteback{
		GoalID: "g", TodoID: "t1", TurnID: "turn-x", AgentID: "a1",
		Outcome:  Outcome{Status: OutcomeProgress},
		Evidence: []Evidence{{ID: "e1", Kind: "diff", Summary: "ok"}},
	})
	require.NoError(t, err)

	// Process B: spend against A's delivery — must succeed (cross-process).
	total, err := kB.SpendSlot(ctx, "g", "turn-x", SpendOpts{Execute: true})
	require.NoError(t, err, "spend in process B must see process A's delivery")
	assert.Equal(t, 1, total)

	// B's second spend is rejected (delivery spent in the shared store).
	_, err = kB.SpendSlot(ctx, "g", "turn-x", SpendOpts{Execute: true})
	assert.Error(t, err)
}

func TestSQLLedgerLastCorrectUnderInterleaving(t *testing.T) {
	// #1: global AUTOINCREMENT seq interleaves events across goals. readRecent
	// must use Last (ORDER BY seq DESC LIMIT n), NOT Len+Read cursor arithmetic.
	db := openDB(t, ":memory:")
	stores, err := NewSQLStores(db)
	require.NoError(t, err)
	led := stores.Ledger
	ctx := context.Background()

	// Interleave A1, B1, A2, B2, A3, B3 — A's seqs are 1,3,5; B's are 2,4,6.
	for _, pair := range []struct{ g, tag string }{
		{"A", "a1"}, {"B", "b1"}, {"A", "a2"}, {"B", "b2"}, {"A", "a3"}, {"B", "b3"},
	} {
		_, err := led.Append(ctx, Event{GoalID: pair.g, Type: pair.tag})
		require.NoError(t, err)
	}

	// Last(A, 2) must be a2,a3 (A's two newest), NOT a1,a2.
	got, err := led.Last(ctx, "A", 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a2", got[0].Type)
	assert.Equal(t, "a3", got[1].Type, "Last must return the NEWEST events under interleaving")

	// Last(B, 2) = b2,b3.
	gotB, _ := led.Last(ctx, "B", 2)
	require.Len(t, gotB, 2)
	assert.Equal(t, "b3", gotB[1].Type)
}

func TestSQLLedgerCompactBoundsGrowth(t *testing.T) {
	db := openDB(t, ":memory:")
	stores, err := NewSQLStores(db)
	require.NoError(t, err)
	led := stores.Ledger
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, _ = led.Append(ctx, Event{GoalID: "g", Type: "w"})
	}
	before, _ := led.Len(ctx, "g")
	require.Equal(t, int64(10), before)
	// Keep only the last 3; the rest fold into one compacted marker -> 4 rows.
	require.NoError(t, led.Compact(ctx, "g", 3))
	after, _ := led.Len(ctx, "g")
	assert.Equal(t, int64(4), after, "compact keeps last 3 + 1 compacted marker")
	last, _ := led.Last(ctx, "g", 1)
	require.Len(t, last, 1)
	// The compacted marker is the most-recent event.
	assert.Equal(t, "history_compacted", last[0].Type)
}

func TestTicketStoreReapAndCompact(t *testing.T) {
	// #5: tickets and ledger events are reaped/compacted to bound growth.
	db := openDB(t, ":memory:")
	require.NoError(t, InitSchema(db)) // ensure cp_tickets exists
	ctx := context.Background()

	// Mint + consume a ticket; then reap consumed -> it's gone.
	ts := NewSQLTicketStore(db)
	tok, err := ts.Mint(ctx, "g", "turn-old")
	require.NoError(t, err)
	require.NoError(t, ts.Consume(ctx, "g", "turn-old", tok))
	// Backdate it so the reap cutoff catches it.
	_, err = db.Exec(`UPDATE cp_tickets SET minted_at = ? WHERE goal_id='g' AND turn_id='turn-old'`,
		ts2(time.Now().UTC().Add(-2*time.Hour)))
	require.NoError(t, err)
	require.NoError(t, ts.Reap(ctx, time.Hour))
	// Consume again -> now reports ErrNoShouldRunTicket (reaped).
	err = ts.Consume(ctx, "g", "turn-old", tok)
	assert.ErrorIs(t, err, ErrNoShouldRunTicket)
}

// ts2 is a tiny RFC3339 formatter local to these tests.
func ts2(t time.Time) string { return t.Format("2006-01-02T15:04:05.999999999Z07:00") }

func TestSQLRunInTxRollsBackOnError(t *testing.T) {
	// #4: a multi-step mutation wrapped in RunInTx must roll back ALL writes if
	// fn returns an error. This is the atomicity guarantee SupersedeTodo relies on.
	db := openDB(t, ":memory:")
	stores, err := NewSQLStores(db)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, stores.Goals.Upsert(ctx, &Goal{ID: "g", Objective: "o", State: GoalActive, Quota: DefaultQuota()}))

	inserted := "rollback-todo"
	err = stores.RunInTx(ctx, func(tctx context.Context) error {
		// First write succeeds (within tx).
		if e := stores.Todos.Upsert(tctx, &Todo{ID: inserted, GoalID: "g", State: TodoOpen}); e != nil {
			return e
		}
		// Simulate a mid-tx failure.
		return errors.New("boom")
	})
	require.Error(t, err)

	// The first write must NOT be visible after rollback.
	_, err = stores.Todos.Get(ctx, "g", inserted)
	assert.ErrorIs(t, err, ErrTodoNotFound, "tx write must be rolled back on fn error")
}

func TestSQLSupersedeTodoIsAtomic(t *testing.T) {
	// #4: SupersedeTodo under SQL advances successor+old+goal atomically.
	k, err := NewSQLKernel(openDB(t, ":memory:"))
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, k.GoalStore().Upsert(ctx, &Goal{ID: "g", Objective: "o", State: GoalActive, Quota: DefaultQuota(), CurrentTodoID: "t1"}))
	require.NoError(t, k.TodoStore().Upsert(ctx, &Todo{ID: "t1", GoalID: "g", State: TodoOpen, TaskClass: TaskAdvancement}))

	succ, err := k.SupersedeTodo(ctx, "g", "t1", "a1", "v2", "")
	require.NoError(t, err)

	// All three writes committed consistently.
	old, _ := k.TodoStore().Get(ctx, "g", "t1")
	assert.Equal(t, TodoDeferred, old.State)
	assert.Equal(t, succ.ID, old.SupersededBy)
	g, _ := k.GoalStore().Get(ctx, "g")
	assert.Equal(t, succ.ID, g.CurrentTodoID, "goal advanced to successor within the tx")
}

func TestSQLLedgerLenAndReadRecent(t *testing.T) {
	// Proves the #2 fix: readRecent returns the LAST n, not the oldest n.
	k := newSQLKernelInMem(t)
	ctx := context.Background()
	require.NoError(t, k.GoalStore().Upsert(ctx, &Goal{ID: "g1", Objective: "x", State: GoalActive, Quota: DefaultQuota()}))
	for i := 0; i < 5; i++ {
		_, err := k.Ledger().Append(ctx, Event{Kind: EventWork, Type: "w", GoalID: "g1", Detail: map[string]any{"i": i}})
		require.NoError(t, err)
	}
	total, err := k.Ledger().Len(ctx, "g1")
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)

	recent := k.readRecent(ctx, "g1", 3)
	require.Len(t, recent, 3)
	// readRecent returns chronological within the window: recent[0]=i2 ... recent[2]=i4.
	// Use the backend-agnostic accessor (#4): Detail values are int on Memory but
	// float64 after the SQL JSON round-trip.
	first, _ := recent[0].DetailInt("i")
	last, _ := recent[2].DetailInt("i")
	assert.Equal(t, 2, first)
	assert.Equal(t, 4, last, "must return the LAST n, not the oldest")
}

func TestSQLSpendRollingWindow(t *testing.T) {
	db := openDB(t, ":memory:")
	stores, err := NewSQLStores(db)
	require.NoError(t, err)
	log := stores.SpendLog
	ctx := context.Background()

	_, err = log.Append(ctx, SpendEvent{GoalID: "g", Slots: 1, SpentAt: time.Now()})
	require.NoError(t, err)
	_, err = log.Append(ctx, SpendEvent{GoalID: "g", Slots: 1, SpentAt: time.Now()})
	require.NoError(t, err)
	n, err := log.SpentInWindow(ctx, "g", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	_, err = log.Append(ctx, SpendEvent{GoalID: "g", Slots: 1, SpentAt: time.Now().Add(-2 * time.Hour)})
	require.NoError(t, err)
	n, _ = log.SpentInWindow(ctx, "g", time.Hour)
	assert.Equal(t, 2, n, "out-of-window spend excluded")
}

func TestSQLGateStoreRoundTrip(t *testing.T) {
	k := newSQLKernelInMem(t)
	ctx := context.Background()
	g := UserGate{
		GateID: "gg", GoalID: "goal", Question: "ok?",
		Scope:     DecisionScope{Kind: ScopeWrite, ScopeKey: "main.go"},
		Resolvers: []string{"alice", "bob"},
	}
	require.NoError(t, k.GateStore().Upsert(ctx, g))
	got, err := k.GateStore().Get(ctx, "goal", "gg")
	require.NoError(t, err)
	assert.Equal(t, "ok?", got.Question)
	assert.Equal(t, []string{"alice", "bob"}, got.Resolvers)

	unresolved, err := k.GateStore().ListUnresolved(ctx, "goal")
	require.NoError(t, err)
	require.Len(t, unresolved, 1)

	g.Outcome = &GateOutcome{Decision: DecisionApprove}
	require.NoError(t, k.GateStore().Upsert(ctx, g))
	unresolved, _ = k.GateStore().ListUnresolved(ctx, "goal")
	assert.Empty(t, unresolved, "resolved gate excluded from unresolved list")
}

func TestCorruptJSONColumnFailsLoudly(t *testing.T) {
	// Corrupt JSON must surface an error on read, NOT silently zero the struct:
	// a corrupted hard_policy scope would otherwise default to ScopeKey=""
	// (a GLOBAL veto in SelectByPrecedence), silently over-broadening policy.
	db := openDB(t, ":memory:")
	stores, err := NewSQLStores(db)
	require.NoError(t, err)
	ctx := context.Background()

	// Write a reward row with corrupt scope JSON directly.
	_, err = db.Exec(`INSERT INTO cp_rewards (id, goal_id, class, scope, confidence, lifecycle, content, at)
		VALUES ('r1', 'g', 'hard_policy', '{{{not-json', 'high', 'active', 'veto', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)

	// Reading must FAIL, not return a scope-less global policy.
	_, err = stores.Rewards.List(ctx, "g")
	require.Error(t, err, "corrupt reward scope must fail loudly")
	assert.Contains(t, err.Error(), "corrupt JSON")

	// Same for a corrupt goal scope.
	_, err = db.Exec(`UPDATE cp_goals SET scope='[bad' WHERE id='missing'`)
	require.NoError(t, err)
}
