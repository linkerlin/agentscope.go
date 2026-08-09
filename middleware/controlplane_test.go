package middleware

import (
	"context"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCPAgent struct{ name string }

func (a *stubCPAgent) AgentName() string { return a.name }

// markerMsg is returned by next() so tests can tell passthrough from block.
func markerMsg() *message.Msg {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("RAN").Build()
}

func setupCPKernel(t *testing.T) *controlplane.Kernel {
	t.Helper()
	goals := controlplane.NewMemoryGoalStore()
	todos := controlplane.NewMemoryTodoStore()
	k := controlplane.NewKernel(goals, todos, nil)
	require.NoError(t, goals.Upsert(context.Background(), &controlplane.Goal{
		ID: "goal-1", Objective: "test", State: controlplane.GoalActive, Quota: controlplane.DefaultQuota(),
	}))
	return k
}

func TestControlPlaneMiddleware_PassthroughWhenNoGoalBound(t *testing.T) {
	k := setupCPKernel(t)
	// GoalResolver returns ok=false -> passthrough.
	mw := &ControlPlaneMiddleware{
		Kernel:       k,
		GoalResolver: func(string) (string, bool) { return "", false },
	}
	called := false
	out, err := mw.OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		called = true
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.True(t, called, "next must run when no goal bound")
	assert.Equal(t, "RAN", out.GetTextContent())
}

func TestControlPlaneMiddleware_PassthroughWhenEligible(t *testing.T) {
	k := setupCPKernel(t)
	mw := &ControlPlaneMiddleware{
		Kernel:       k,
		GoalResolver: func(string) (string, bool) { return "goal-1", true },
	}
	called := false
	out, err := mw.OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		called = true
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.True(t, called, "next must run when eligible")
	assert.Equal(t, "RAN", out.GetTextContent())
}

func TestControlPlaneMiddleware_BlockedByGate(t *testing.T) {
	k := setupCPKernel(t)
	require.NoError(t, k.OpenGate(context.Background(), controlplane.UserGate{
		GateID: "g-1", GoalID: "goal-1", Question: "Approve deploy to prod?",
		Scope: controlplane.DecisionScope{Kind: controlplane.ScopeProduction, ScopeKey: "prod"},
	}))
	mw := &ControlPlaneMiddleware{
		Kernel:       k,
		GoalResolver: func(string) (string, bool) { return "goal-1", true },
	}
	called := false
	out, err := mw.OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		called = true
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.False(t, called, "next must NOT run when gate blocks")
	require.NotNil(t, out)
	txt := out.GetTextContent()
	assert.Contains(t, txt, "Approve deploy to prod?")
	assert.Contains(t, txt, "g-1")
}

func TestControlPlaneMiddleware_BlockedWhenPaused(t *testing.T) {
	k := setupCPKernel(t)
	g, _ := k.GoalStore().Get(context.Background(), "goal-1")
	g.State = controlplane.GoalPaused
	require.NoError(t, k.GoalStore().Upsert(context.Background(), g))

	mw := &ControlPlaneMiddleware{
		Kernel:       k,
		GoalResolver: func(string) (string, bool) { return "goal-1", true },
	}
	called := false
	out, err := mw.OnReply(context.Background(), &stubCPAgent{"a1"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		called = true
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.Contains(t, out.GetTextContent(), "paused")
}

func TestControlPlaneMiddleware_AgentIDFromName(t *testing.T) {
	k := setupCPKernel(t)
	seen := ""
	mw := &ControlPlaneMiddleware{
		Kernel: k,
		// Bind goal only for the mapped id "real-agent".
		GoalResolver: func(id string) (string, bool) {
			seen = id
			if id == "real-agent" {
				return "goal-1", true
			}
			return "", false
		},
		AgentIDFromName: func(name string) string { return "real-agent" },
	}
	out, err := mw.OnReply(context.Background(), &stubCPAgent{"display-name"}, &ReplyInput{}, func(ctx context.Context) (*message.Msg, error) {
		return markerMsg(), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "real-agent", seen, "AgentIDFromName must rewrite the id before resolution")
	// Eligible -> next ran -> RAN.
	assert.Equal(t, "RAN", out.GetTextContent())
}

func TestBlockTextFallbackReason(t *testing.T) {
	// Sanity: a decision with no recognized state but a Reason still renders.
	txt := blockText(&controlplane.Decision{
		State: controlplane.ComputeBlockedHealth, Reason: "registry unhealthy",
	})
	assert.True(t, strings.HasPrefix(txt, "[control-plane]"))
	assert.Contains(t, txt, "registry unhealthy")
}
