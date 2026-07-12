package messagebus_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/linkerlin/agentscope.go/messagebus"
	bredis "github.com/redis/go-redis/v9"
)

func miniredisBus(t *testing.T) (*messagebus.RedisBus, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := bredis.NewClient(&bredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return messagebus.NewRedisBus(client, "test"), mr
}

func TestRedisCoord_LockRelease(t *testing.T) {
	bus, _ := miniredisBus(t)
	ctx := context.Background()

	release, err := bus.Lock(ctx, "res", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Same key held: a second acquire with a short deadline must fail.
	ctx2, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if _, err := bus.Lock(ctx2, "res", 0); err == nil {
		t.Fatal("second lock should not acquire while held")
	}
	release()
	// After release it acquires again.
	if _, err := bus.Lock(ctx, "res", 0); err != nil {
		t.Fatalf("lock failed after release: %v", err)
	}
}

func TestRedisCoord_LockTTL(t *testing.T) {
	bus, mr := miniredisBus(t)
	ctx := context.Background()
	if _, err := bus.Lock(ctx, "r", 80*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// Advance miniredis's clock past the TTL (real time.Sleep does not reliably
	// expire keys in miniredis).
	mr.FastForward(150 * time.Millisecond)
	// Guard against an infinite poll loop with a deadline.
	acquireCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := bus.Lock(acquireCtx, "r", 0); err != nil {
		t.Fatalf("lock not auto-released: %v", err)
	}
}

func TestRedisCoord_Registry(t *testing.T) {
	bus, _ := miniredisBus(t)
	ctx := context.Background()
	if err := bus.RegistrySet(ctx, "ns", "k1", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	v, err := bus.RegistryGet(ctx, "ns", "k1")
	if err != nil || string(v) != "v1" {
		t.Fatalf("get: %v %q", err, v)
	}
	bus.RegistrySet(ctx, "ns", "k2", []byte("v2"))
	list, _ := bus.RegistryList(ctx, "ns")
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	bus.RegistryDelete(ctx, "ns", "k1")
	list, _ = bus.RegistryList(ctx, "ns")
	if len(list) != 1 || string(list["k2"]) != "v2" {
		t.Fatalf("after delete: %+v", list)
	}
}

func TestRedisCoord_Queue(t *testing.T) {
	bus, _ := miniredisBus(t)
	ctx := context.Background()
	bus.QueuePush(ctx, "jobs", []byte("a"))
	bus.QueuePush(ctx, "jobs", []byte("b"))
	v, err := bus.QueuePop(ctx, "jobs")
	if err != nil || string(v) != "a" {
		t.Fatalf("pop1: %v %q", err, v)
	}
	v, err = bus.QueuePop(ctx, "jobs")
	if err != nil || string(v) != "b" {
		t.Fatalf("pop2: %v %q", err, v)
	}
}

func TestRedisCoord_QueuePopCtxCancel(t *testing.T) {
	bus, _ := miniredisBus(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_, err := bus.QueuePop(ctx, "empty")
	if err == nil {
		t.Fatal("expected error on empty queue timeout")
	}
}

func TestRedisCoord_Log(t *testing.T) {
	bus, _ := miniredisBus(t)
	ctx := context.Background()
	i1, _ := bus.LogAppend(ctx, "audit", []byte("x"))
	i2, _ := bus.LogAppend(ctx, "audit", []byte("y"))
	if i1 != 0 || i2 != 1 {
		t.Fatalf("indices: %d %d", i1, i2)
	}
	entries, next, _ := bus.LogRead(ctx, "audit", 0, 10)
	if len(entries) != 2 || string(entries[0]) != "x" || string(entries[1]) != "y" {
		t.Fatalf("read: %v", entries)
	}
	if next != 2 {
		t.Fatalf("next = %d", next)
	}
}

func TestRedisCoord_AsCoordBus(t *testing.T) {
	bus, _ := miniredisBus(t)
	if messagebus.AsCoordBus(bus) == nil {
		t.Fatal("RedisBus should satisfy CoordBus")
	}
}
