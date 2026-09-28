package messagebus

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLocalLease_ExclusiveAndRelease(t *testing.T) {
	bus := NewLocalBus()
	ctx := context.Background()

	l1, cancel1, err := bus.AcquireLease(ctx, "res", "owner-a", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if l1.Token == 0 {
		t.Fatal("fencing token must be positive")
	}

	if _, _, err := bus.AcquireLease(ctx, "res", "owner-b", 5*time.Second); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("second acquire: want ErrLeaseHeld, got %v", err)
	}

	cancel1()
	if _, _, err := bus.AcquireLease(ctx, "res", "owner-b", 5*time.Second); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestLocalLease_RenewCAS(t *testing.T) {
	bus := NewLocalBus()
	ctx := context.Background()

	l1, cancel1, err := bus.AcquireLease(ctx, "res", "owner-a", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel1()

	// Live renewal succeeds and extends expiry.
	deadline := l1.Expires
	if err := bus.RenewLease(ctx, "res", l1, 2*time.Second); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !l1.Expires.After(deadline) {
		t.Fatal("renew did not extend expiry")
	}

	// After release, renewal loses the fence.
	cancel1()
	if err := bus.RenewLease(ctx, "res", l1, time.Second); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("renew after release: want ErrLeaseLost, got %v", err)
	}
}

func TestLocalLease_ExpiredTakeoverBumpsFence(t *testing.T) {
	bus := NewLocalBus()
	ctx := context.Background()

	l1, cancel1, err := bus.AcquireLease(ctx, "res", "owner-a", 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel1()

	time.Sleep(60 * time.Millisecond) // let it expire

	// A stale holder's renewal must fail.
	if err := bus.RenewLease(ctx, "res", l1, time.Second); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale renew: want ErrLeaseLost, got %v", err)
	}

	// Takeover succeeds with a strictly greater token.
	l2, cancel2, err := bus.AcquireLease(ctx, "res", "owner-b", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel2()
	if l2.Token <= l1.Token {
		t.Fatalf("fencing token not monotonic: %d after %d", l2.Token, l1.Token)
	}

	// The stale holder's release must not evict the new lease.
	cancel1()
	if err := bus.RenewLease(ctx, "res", l2, time.Second); err != nil {
		t.Fatalf("successor lease evicted by stale release: %v", err)
	}
}

func TestLocalLease_RenewedLeaseSurvivesStaleTimer(t *testing.T) {
	bus := NewLocalBus()
	ctx := context.Background()

	l1, cancel, err := bus.AcquireLease(ctx, "res", "owner-a", 40*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	// Renew past the original TTL several times; the original reaper timer
	// must never remove the renewed record.
	for i := 0; i < 4; i++ {
		time.Sleep(25 * time.Millisecond)
		if err := bus.RenewLease(ctx, "res", l1, 40*time.Millisecond); err != nil {
			t.Fatalf("renew %d: %v", i, err)
		}
	}
	bus.mu.RLock()
	_, held := bus.leases["res"]
	bus.mu.RUnlock()
	if !held {
		t.Fatal("renewed lease was removed by a stale expiry timer")
	}
}

func TestLocalRegistry_CompareAndDelete(t *testing.T) {
	bus := NewLocalBus()
	ctx := context.Background()

	if err := bus.RegistrySet(ctx, "ns", "k", []byte("v1")); err != nil {
		t.Fatal(err)
	}

	// Wrong expectation: no delete.
	ok, err := bus.RegistryCompareAndDelete(ctx, "ns", "k", []byte("other"))
	if err != nil || ok {
		t.Fatalf("CAS with wrong value: deleted=%v err=%v", ok, err)
	}
	if _, err := bus.RegistryGet(ctx, "ns", "k"); err != nil {
		t.Fatal("entry was deleted despite CAS mismatch")
	}

	// Right expectation: deletes.
	ok, err = bus.RegistryCompareAndDelete(ctx, "ns", "k", []byte("v1"))
	if err != nil || !ok {
		t.Fatalf("CAS with right value: deleted=%v err=%v", ok, err)
	}
	if _, err := bus.RegistryGet(ctx, "ns", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("entry still present after CAS delete: %v", err)
	}
}

func TestAsCoordLeaseViews(t *testing.T) {
	if AsCoordLease(NewLocalBus()) == nil {
		t.Fatal("LocalBus must implement CoordLease")
	}
	if AsRegistryCAS(NewLocalBus()) == nil {
		t.Fatal("LocalBus must implement RegistryCAS")
	}
}
