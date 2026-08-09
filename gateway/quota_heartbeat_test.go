package gateway

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaHeartbeatEligibleRunsTurn(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil).WithTicketEnforcement()
	ctx := context.Background()
	require.NoError(t, k.GoalStore().Upsert(ctx, &controlplane.Goal{
		ID: "g", Objective: "o", State: controlplane.GoalActive, Quota: controlplane.DefaultQuota(),
	}))
	require.NoError(t, k.TodoStore().Upsert(ctx, &controlplane.Todo{ID: "t", GoalID: "g", State: controlplane.TodoOpen, TaskClass: controlplane.TaskAdvancement}))

	ran := false
	h := &QuotaHeartbeat{
		Kernel: k, GoalID: "g", AgentID: "worker",
		RunTurn: func(ctx context.Context, token string) error {
			ran = true
			require.NotEmpty(t, token, "bounded turn receives the minted token")
			return nil
		},
	}
	dec, err := h.Tick(ctx, "turn-1")
	require.NoError(t, err)
	assert.True(t, dec.ShouldRun)
	assert.True(t, ran, "eligible heartbeat runs the bounded turn")
}

func TestQuotaHeartbeatQuietSkipsWhenBlocked(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	ctx := context.Background()
	require.NoError(t, k.GoalStore().Upsert(ctx, &controlplane.Goal{
		ID: "g", Objective: "o", State: controlplane.GoalActive, Quota: controlplane.DefaultQuota(),
	}))
	// Open a gate -> ShouldRun blocks.
	require.NoError(t, k.OpenGate(ctx, controlplane.UserGate{
		GateID: "gg", GoalID: "g", Question: "hold",
		Scope: controlplane.DecisionScope{Kind: controlplane.ScopeWrite, ScopeKey: "x"},
	}))

	ran := false
	h := &QuotaHeartbeat{
		Kernel: k, GoalID: "g", AgentID: "worker",
		RunTurn: func(ctx context.Context, token string) error { ran = true; return nil },
	}
	dec, err := h.Tick(ctx, "turn-1")
	require.NoError(t, err)
	assert.False(t, dec.ShouldRun, "gate blocks the heartbeat")
	assert.False(t, ran, "blocked heartbeat is a quiet skip: no work, no spend")
}

func TestQuotaHeartbeatPausedSkips(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	ctx := context.Background()
	require.NoError(t, k.GoalStore().Upsert(ctx, &controlplane.Goal{
		ID: "g", Objective: "o", State: controlplane.GoalPaused, Quota: controlplane.DefaultQuota(),
	}))
	h := &QuotaHeartbeat{Kernel: k, GoalID: "g", AgentID: "worker"}
	dec, err := h.Tick(ctx, "turn-1")
	require.NoError(t, err)
	assert.False(t, dec.ShouldRun)
	assert.Equal(t, controlplane.ComputePaused, dec.State)
}
