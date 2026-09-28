package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leaseTestBus returns a bus whose CoordLease/RegistryCAS views the
// coordinator consumes, plus a raw lease handle for out-of-band takeover
// scenarios.
func leaseTestCoordinator(t *testing.T, leaseTTL time.Duration) (*SessionCoordinator, messagebus.Bus) {
	t.Helper()
	bus := messagebus.NewLocalBus()
	c := NewSessionCoordinator(NewSessionManager()).WithBus(bus).WithLeaseTTL(leaseTTL)
	require.NotNil(t, c.lease, "LocalBus must expose fencing leases")
	return c, bus
}

// TestSessionCoordinator_LeaseKeepsLongTurnExclusive locks the 23.1 rule:
// a long turn under a short lease TTL runs to completion on one replica —
// the renewal pump keeps the fence alive, and a second replica is refused
// (busy) for the whole duration, not just for the first TTL slice.
func TestSessionCoordinator_LeaseKeepsLongTurnExclusive(t *testing.T) {
	c1, bus := leaseTestCoordinator(t, 90*time.Millisecond)
	other := NewSessionCoordinator(NewSessionManager()).WithBus(bus).WithLeaseTTL(90 * time.Millisecond)

	// Replica 1 runs a turn far longer than the lease TTL.
	a := makeMockAgent(turnEvents("r1"), 600*time.Millisecond)
	ch, err := c1.Run(ctxAsUser("u-a"), "s-lease", a, turnMsg())
	require.NoError(t, err)

	// Throughout the turn, other replicas see busy.
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, err := other.Run(ctxAsUser("u-a"), "s-lease", makeMockAgent(turnEvents("x"), time.Millisecond), turnMsg())
		assert.ErrorIs(t, err, ErrSessionBusy, "second replica must be busy during a renewed long turn")
		time.Sleep(40 * time.Millisecond)
	}

	drain(ch)

	// After the run the lease is released: another replica can take over.
	_, err = other.Run(ctxAsUser("u-a"), "s-lease", makeMockAgent(turnEvents("r2"), time.Millisecond), turnMsg())
	require.NoError(t, err, "takeover after completion must succeed")
}

// TestSessionCoordinator_LeaseTakeoverBumpsFenceInRegistry locks the 23.1
// rule: the running registry marker carries the fencing token, and a
// takeover publishes a strictly greater fence.
func TestSessionCoordinator_LeaseTakeoverBumpsFenceInRegistry(t *testing.T) {
	c1, bus := leaseTestCoordinator(t, 60*time.Millisecond)
	cb := messagebus.AsCoordBus(bus)
	ns := c1.keys.SessionRunRegistryNS()

	a := makeMockAgent(turnEvents("r1"), 40*time.Millisecond)
	ch, err := c1.Run(ctxAsUser("u-a"), "s-fence", a, turnMsg())
	require.NoError(t, err)

	// While running, the marker carries owner + fence.
	var m1 leaseMarker
	require.Eventually(t, func() bool {
		raw, err := cb.RegistryGet(context.Background(), ns, scopedSessionID(ctxAsUser("u-a"), "s-fence"))
		if err != nil {
			return false
		}
		return json.Unmarshal(raw, &m1) == nil && m1.Fence > 0
	}, time.Second, 10*time.Millisecond)

	drain(ch)

	// Take over: the new marker's fence is strictly greater.
	a2 := makeMockAgent(turnEvents("r2"), 20*time.Millisecond)
	ch2, err := c1.Run(ctxAsUser("u-a"), "s-fence", a2, turnMsg())
	require.NoError(t, err)
	var m2 leaseMarker
	require.Eventually(t, func() bool {
		raw, err := cb.RegistryGet(context.Background(), ns, scopedSessionID(ctxAsUser("u-a"), "s-fence"))
		if err != nil {
			return false
		}
		return json.Unmarshal(raw, &m2) == nil
	}, time.Second, 10*time.Millisecond)
	assert.Greater(t, m2.Fence, m1.Fence, "takeover must bump the fencing token")

	drain(ch2)

	// CAS cleanup: the finished run leaves no marker behind.
	_, err = cb.RegistryGet(context.Background(), ns, scopedSessionID(ctxAsUser("u-a"), "s-fence"))
	assert.ErrorIs(t, err, messagebus.ErrNotFound, "run end must CAS-delete its own marker")
}

// TestSessionCoordinator_FenceLossTerminatesRun locks the 23.1 rule: a
// holder that loses its lease (stalled past TTL, another replica took over)
// must terminate its local run immediately instead of double-executing.
func TestSessionCoordinator_FenceLossTerminatesRun(t *testing.T) {
	c1, bus := leaseTestCoordinator(t, 80*time.Millisecond)
	rawLease := messagebus.AsCoordLease(bus)
	// Simulate a stalled holder: the renewal pump ticks far less often than
	// the lease TTL, so the lease expires mid-turn (as if the replica froze
	// or partitioned) and a raider takes over.
	c1.renewInterval = 300 * time.Millisecond

	a := makeMockAgent(turnEvents("r1"), 2*time.Second) // long turn
	ch, err := c1.Run(ctxAsUser("u-a"), "s-loss", a, turnMsg())
	require.NoError(t, err)

	// After expiry (80ms), the raider takes over out-of-band.
	require.Eventually(t, func() bool {
		_, _, err := rawLease.AcquireLease(context.Background(),
			c1.keys.SessionRunLockKey(scopedSessionID(ctxAsUser("u-a"), "s-loss")), "raider", 5*time.Second)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)

	// c1's renewal pump (t≈300ms) now hits ErrLeaseLost and must terminate
	// the local run promptly — long before the turn's natural 2s end.
	select {
	case <-ch:
		// Terminated after losing the fence.
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("run was not terminated after losing its fence")
	}
}

// TestSessionCoordinator_LeaseStatusAndCancelStillWork verifies the 18.2
// status endpoint and cross-replica cancel keep working on the lease path.
func TestSessionCoordinator_LeaseStatusAndCancelStillWork(t *testing.T) {
	c1, bus := leaseTestCoordinator(t, 500*time.Millisecond)
	other := NewSessionCoordinator(NewSessionManager()).WithBus(bus).WithLeaseTTL(500 * time.Millisecond)

	ctxA := ctxAsUser("u-a")
	a := makeMockAgent(turnEvents("r1"), 400*time.Millisecond)
	ch, err := c1.Run(ctxA, "s-status", a, turnMsg())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return other.Status(ctxA, "s-status") == StatusRunning
	}, time.Second, 10*time.Millisecond)

	// Cross-replica cancel resolves through the owner's scoped channel.
	require.NoError(t, other.Cancel(ctxA, "s-status"))
	drain(ch)
}
