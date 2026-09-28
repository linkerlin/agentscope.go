package messagebus

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTestClient dials $REDIS_URL for the fencing-lease integration tests.
// Skips when REDIS_URL is unset or the server is unreachable — the same
// convention as redis_team_test.go.
func redisTestClient(t *testing.T) *redis.Client {
	t.Helper()
	raw := os.Getenv("REDIS_URL")
	if raw == "" {
		t.Skip("REDIS_URL not set; skipping Redis integration test")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse REDIS_URL %q: %v", raw, err)
	}
	opts := &redis.Options{Addr: u.Host}
	if u.User != nil {
		opts.Password, _ = u.User.Password()
	}
	client := redis.NewClient(opts)
	ping, err := client.Ping(context.Background()).Result()
	if err != nil {
		client.Close()
		t.Skipf("redis unreachable at %s: %v", raw, err)
	}
	_ = ping
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRedisLease_ExclusiveRenewTakeover(t *testing.T) {
	client := redisTestClient(t)
	ctx := context.Background()
	bus := NewRedisBus(client, "lease-test-"+time.Now().Format("150405.000000000"))

	l1, cancel1, err := bus.AcquireLease(ctx, "res", "owner-a", 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if l1.Token == 0 {
		t.Fatal("fencing token must be positive")
	}

	// Held: second owner refused.
	if _, _, err := bus.AcquireLease(ctx, "res", "owner-b", time.Second); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("want ErrLeaseHeld, got %v", err)
	}

	// Live renewal succeeds.
	if err := bus.RenewLease(ctx, "res", l1, 300*time.Millisecond); err != nil {
		t.Fatalf("renew: %v", err)
	}

	// Expire, then a stale renewal fails and takeover bumps the fence.
	time.Sleep(400 * time.Millisecond)
	if err := bus.RenewLease(ctx, "res", l1, time.Second); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale renew: want ErrLeaseLost, got %v", err)
	}
	l2, cancel2, err := bus.AcquireLease(ctx, "res", "owner-b", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel2()
	if l2.Token <= l1.Token {
		t.Fatalf("fencing token not monotonic: %d after %d", l2.Token, l1.Token)
	}

	// Stale release must not evict the successor.
	cancel1()
	if err := bus.RenewLease(ctx, "res", l2, time.Second); err != nil {
		t.Fatalf("successor lease evicted by stale release: %v", err)
	}
}

func TestRedisLease_ConcurrentTakeoverSingleWinner(t *testing.T) {
	client := redisTestClient(t)
	ctx := context.Background()
	bus := NewRedisBus(client, "lease-race-"+time.Now().Format("150405.000000000"))

	// Ten concurrent acquirers on a fresh key: exactly one wins.
	const contenders = 10
	won := make(chan *Lease, contenders)
	failed := make(chan error, contenders)
	for i := 0; i < contenders; i++ {
		owner := "owner-" + time.Now().Format("150405.000000") + "-" + string(rune('a'+i))
		go func() {
			l, cancel, err := bus.AcquireLease(ctx, "contended", owner, time.Second)
			if err != nil {
				failed <- err
				return
			}
			defer cancel()
			won <- l
		}()
	}
	winners, losers := 0, 0
	for i := 0; i < contenders; i++ {
		select {
		case <-won:
			winners++
		case <-failed:
			losers++
		}
	}
	if winners != 1 || losers != contenders-1 {
		t.Fatalf("contended acquire: %d winners, %d losers (want 1/%d)", winners, losers, contenders-1)
	}
}

func TestRedisRegistry_CompareAndDelete(t *testing.T) {
	client := redisTestClient(t)
	ctx := context.Background()
	bus := NewRedisBus(client, "lease-cas-"+time.Now().Format("150405.000000000"))

	if err := bus.RegistrySet(ctx, "ns", "k", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	ok, err := bus.RegistryCompareAndDelete(ctx, "ns", "k", []byte("other"))
	if err != nil || ok {
		t.Fatalf("CAS with wrong value: deleted=%v err=%v", ok, err)
	}
	ok, err = bus.RegistryCompareAndDelete(ctx, "ns", "k", []byte("v1"))
	if err != nil || !ok {
		t.Fatalf("CAS with right value: deleted=%v err=%v", ok, err)
	}
}
