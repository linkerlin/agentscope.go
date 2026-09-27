package gateway

import (
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionCoordinator_TenantScopedLocks locks the 22.2 rule: two tenants
// referencing the same external session ID coordinate under disjoint keys —
// one tenant's running turn never blocks or 409s the other tenant.
func TestSessionCoordinator_TenantScopedLocks(t *testing.T) {
	bus := messagebus.NewLocalBus()
	c1 := NewSessionCoordinator(NewSessionManager()).WithBus(bus).
		WithLockAcquireTimeout(50 * time.Millisecond)
	c2 := NewSessionCoordinator(NewSessionManager()).WithBus(bus).
		WithLockAcquireTimeout(50 * time.Millisecond)

	// Tenant A starts a slow run on session "shared-id".
	ctxA := ctxAsUser("u-a")
	a := makeMockAgent(turnEvents("r1"), 300*time.Millisecond)
	chA, err := c1.Run(ctxA, "shared-id", a, turnMsg())
	require.NoError(t, err)

	// Same external session ID, different tenant: must NOT be busy — the
	// lock keys are tenant-scoped, so this is a genuinely different session.
	ctxB := ctxAsUser("u-b")
	b := makeMockAgent(turnEvents("r2"), 60*time.Millisecond)
	chB, err := c2.Run(ctxB, "shared-id", b, turnMsg())
	require.NoError(t, err, "tenant B must not contend with tenant A on the same external ID")
	drain(chB)

	// Tenant A's run is still going; a same-tenant second run IS busy.
	_, err = c1.Run(ctxA, "shared-id", makeMockAgent(turnEvents("r3"), 60*time.Millisecond), turnMsg())
	assert.ErrorIs(t, err, ErrSessionBusy, "same tenant re-entry must still contend")

	drain(chA)
}

// TestSessionCoordinator_TenantScopedStatusAndCancel verifies status lookups
// and cross-replica cancel also resolve through tenant-scoped keys.
func TestSessionCoordinator_TenantScopedStatusAndCancel(t *testing.T) {
	bus := messagebus.NewLocalBus()
	owner := NewSessionCoordinator(NewSessionManager()).WithBus(bus)
	other := NewSessionCoordinator(NewSessionManager()).WithBus(bus)

	ctxA := ctxAsUser("u-a")
	ctxB := ctxAsUser("u-b")

	a := makeMockAgent(turnEvents("r1"), 150*time.Millisecond)
	ch, err := owner.Run(ctxA, "dup-id", a, turnMsg())
	require.NoError(t, err)

	// Tenant B's status on the same external ID: unknown (its scoped keys
	// hold nothing), never tenant A's running state.
	if got := other.Status(ctxB, "dup-id"); got != StatusUnknown {
		t.Fatalf("tenant B status on foreign ID: got %v, want unknown", got)
	}
	// Tenant A's own status sees the run.
	if got := owner.Status(ctxA, "dup-id"); got != StatusRunning {
		t.Fatalf("owner status: got %v, want running", got)
	}

	// Cross-replica cancel resolves through the owner's scoped channel.
	require.NoError(t, other.Cancel(ctxA, "dup-id"))
	drain(ch)
}
