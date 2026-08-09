package coordbuslease

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/controlplane"
	"github.com/linkerlin/agentscope.go/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAdapter builds an Adapter over a fresh in-process LocalBus CoordBus.
func newAdapter(t *testing.T) (*Adapter, messagebus.CoordBus) {
	t.Helper()
	bus := messagebus.NewLocalBus()
	cb := messagebus.AsCoordBus(bus)
	require.NotNil(t, cb)
	return New(cb), cb
}

func TestAcquireRelease(t *testing.T) {
	ctx := context.Background()
	a, _ := newAdapter(t)

	l, err := a.Acquire(ctx, "g", "t1", "owner1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "owner1", l.Owner)
	assert.True(t, l.IsValid(time.Now()))

	// Inspect sees it.
	got, err := a.Inspect(ctx, "g", "t1")
	require.NoError(t, err)
	assert.Equal(t, "owner1", got.Owner)

	// Release by owner; inspect then not found.
	require.NoError(t, a.Release(ctx, "g", "t1", "owner1"))
	_, err = a.Inspect(ctx, "g", "t1")
	assert.ErrorIs(t, err, controlplane.ErrLeaseNotFound)
}

func TestAcquireCASConflict(t *testing.T) {
	ctx := context.Background()
	a, _ := newAdapter(t)

	_, err := a.Acquire(ctx, "g", "t", "ownerA", time.Minute)
	require.NoError(t, err)

	_, err = a.Acquire(ctx, "g", "t", "ownerB", time.Minute)
	assert.ErrorIs(t, err, controlplane.ErrLeaseHeld)

	// Different todo is independent.
	_, err = a.Acquire(ctx, "g", "t2", "ownerB", time.Minute)
	require.NoError(t, err)
}

func TestOwnerGuards(t *testing.T) {
	ctx := context.Background()
	a, _ := newAdapter(t)
	_, err := a.Acquire(ctx, "g", "t", "owner1", time.Minute)
	require.NoError(t, err)

	// Renew by non-owner.
	_, err = a.Renew(ctx, "g", "t", "intruder", time.Minute)
	assert.ErrorIs(t, err, controlplane.ErrLeaseOwnerMismatch)

	// Release by non-owner.
	err = a.Release(ctx, "g", "t", "intruder")
	assert.ErrorIs(t, err, controlplane.ErrLeaseOwnerMismatch)

	// Renew by owner extends.
	l, err := a.Renew(ctx, "g", "t", "owner1", 2*time.Minute)
	require.NoError(t, err)
	assert.True(t, l.ExpiresAt.After(time.Now().Add(time.Minute)))

	// Transfer to owner2; owner2 can release.
	_, err = a.Transfer(ctx, "g", "t", "owner1", "owner2")
	require.NoError(t, err)
	require.NoError(t, a.Release(ctx, "g", "t", "owner2"))
}

func TestInspectMissingAndExpired(t *testing.T) {
	ctx := context.Background()
	a, _ := newAdapter(t)

	// Missing -> ErrLeaseNotFound.
	_, err := a.Inspect(ctx, "g", "ghost")
	assert.ErrorIs(t, err, controlplane.ErrLeaseNotFound)

	// Expired record is reaped on inspect.
	require.NoError(t, a.writeRec(ctx, "g\x00ghost2", leaseRec{
		Owner: "x", ExpiresAt: time.Now().Add(-time.Second),
	}))
	_, err = a.Inspect(ctx, "g", "ghost2")
	assert.ErrorIs(t, err, controlplane.ErrLeaseNotFound)
}

func TestFastPathSameOwnerReacquire(t *testing.T) {
	ctx := context.Background()
	a, _ := newAdapter(t)

	_, err := a.Acquire(ctx, "g", "t", "owner1", time.Minute)
	require.NoError(t, err)
	// Same owner re-acquires (idempotent renew, not ErrLeaseHeld).
	l2, err := a.Acquire(ctx, "g", "t", "owner1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "owner1", l2.Owner)
}

func TestAdapterInjectsIntoKernel(t *testing.T) {
	// The adapter must satisfy controlplane.LeaseStore and inject via
	// WithLeaseStore, so a Kernel can use CoordBus-backed leases transparently.
	ctx := context.Background()
	a, _ := newAdapter(t)

	k := controlplane.NewKernel(nil, nil, nil).WithLeaseStore(a)
	require.NotNil(t, k)

	l, err := k.AcquireLease(ctx, "g", "t", "agent", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "agent", l.Owner)

	// Another owner blocked via the Kernel too.
	_, err = k.AcquireLease(ctx, "g", "t", "agent2", time.Minute)
	assert.True(t, errors.Is(err, controlplane.ErrLeaseHeld))
}
