package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCPWithCurrentTodo wires goal-1 with a current todo so successful turns
// have an accountable target to write back to (16.4 governance hot path).
func setupCPWithCurrentTodo(t *testing.T) *controlplane.Kernel {
	t.Helper()
	k := setupCPKernel(t)
	require.NoError(t, k.TodoStore().Upsert(context.Background(), &controlplane.Todo{
		ID: "todo-1", GoalID: "goal-1", Description: "do work", State: controlplane.TodoOpen,
	}))
	g, err := k.GoalStore().Get(context.Background(), "goal-1")
	require.NoError(t, err)
	g.CurrentTodoID = "todo-1"
	require.NoError(t, k.GoalStore().Upsert(context.Background(), g))
	return k
}

func spendInWindow(t *testing.T, k *controlplane.Kernel) int {
	t.Helper()
	n, err := k.SpendLog().SpentInWindow(context.Background(), "goal-1", time.Hour)
	require.NoError(t, err)
	return n
}

func boundMW(k *controlplane.Kernel) *ControlPlaneMiddleware {
	return &ControlPlaneMiddleware{
		Kernel:       k,
		GoalResolver: func(string) (string, bool) { return "goal-1", true },
	}
}

// TestAccountTurn_SuccessWritesBackAndSpendsOnce verifies the hot path: a
// successful turn records one evidenced writeback and spends exactly one slot.
func TestAccountTurn_SuccessWritesBackAndSpendsOnce(t *testing.T) {
	k := setupCPWithCurrentTodo(t)
	out, err := boundMW(k).OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "RAN", out.GetTextContent())

	td, err := k.TodoStore().Get(context.Background(), "goal-1", "todo-1")
	require.NoError(t, err)
	require.Len(t, td.Evidence, 1, "writeback must attach one evidence item")
	assert.Equal(t, "turn_reply", td.Evidence[0].Kind)
	assert.Contains(t, td.Evidence[0].Summary, "RAN")
	assert.Equal(t, 1, spendInWindow(t, k), "successful turn spends exactly one slot")
}

// TestAccountTurn_FailureDoesNotSpend verifies "failed attempts do not spend":
// a turn that errors never writes back and never spends.
func TestAccountTurn_FailureDoesNotSpend(t *testing.T) {
	k := setupCPWithCurrentTodo(t)
	_, err := boundMW(k).OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return nil, errors.New("model exploded")
	})
	require.Error(t, err)

	td, gerr := k.TodoStore().Get(context.Background(), "goal-1", "todo-1")
	require.NoError(t, gerr)
	assert.Empty(t, td.Evidence, "failed turn must not write back")
	assert.Equal(t, 0, spendInWindow(t, k), "failed turn must not spend")
}

// TestAccountTurn_NoCurrentTodoSkips verifies a bound goal without a current
// todo has nothing accountable to record: no evidence, no spend, no error.
func TestAccountTurn_NoCurrentTodoSkips(t *testing.T) {
	k := setupCPKernel(t) // no todo wired
	out, err := boundMW(k).OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "RAN", out.GetTextContent())
	assert.Equal(t, 0, spendInWindow(t, k))
}

// TestAccountTurn_WritebackFailureDoesNotSpend verifies "success spends once":
// when the writeback is rejected (claim owner mismatch), no spend may happen.
func TestAccountTurn_WritebackFailureDoesNotSpend(t *testing.T) {
	k := setupCPWithCurrentTodo(t)
	// Another agent holds the claim; our writeback must be rejected.
	td, err := k.TodoStore().Get(context.Background(), "goal-1", "todo-1")
	require.NoError(t, err)
	td.ClaimedBy = "someone-else"
	require.NoError(t, k.TodoStore().Upsert(context.Background(), td))

	out, err := boundMW(k).OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return markerMsg(), nil
	})
	require.NoError(t, err, "accounting failure must not break the reply")
	assert.Equal(t, "RAN", out.GetTextContent())
	assert.Equal(t, 0, spendInWindow(t, k), "rejected writeback must not spend")
}

// TestAccountTurn_UnboundStaysZeroChange verifies unbound sessions keep the
// pre-16.4 behavior: passthrough, no store activity at all.
func TestAccountTurn_UnboundStaysZeroChange(t *testing.T) {
	k := setupCPWithCurrentTodo(t)
	mw := &ControlPlaneMiddleware{
		Kernel:       k,
		GoalResolver: func(string) (string, bool) { return "", false },
	}
	out, err := mw.OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "RAN", out.GetTextContent())
	assert.Equal(t, 0, spendInWindow(t, k))
	td, gerr := k.TodoStore().Get(context.Background(), "goal-1", "todo-1")
	require.NoError(t, gerr)
	assert.Empty(t, td.Evidence)
}
