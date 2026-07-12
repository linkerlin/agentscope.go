package messagebus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalCoord_LockExclusive(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()

	release, err := b.Lock(ctx, "res1", 0)
	if err != nil {
		t.Fatal(err)
	}
	// A second acquirer on the same key must block.
	acquired := make(chan error, 1)
	go func() {
		_, err := b.Lock(context.Background(), "res1", 0)
		acquired <- err
	}()
	select {
	case err := <-acquired:
		t.Fatalf("second lock acquired before release: %v", err)
	case <-time.After(100 * time.Millisecond):
		// expected: still blocked
	}
	release()
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("second lock failed after release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second lock did not acquire after release")
	}
}

func TestLocalCoord_LockTTLAutoRelease(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()
	if _, err := b.Lock(ctx, "r", 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// After TTL expires, a new acquirer should succeed.
	time.Sleep(120 * time.Millisecond)
	release, err := b.Lock(ctx, "r", 0)
	if err != nil {
		t.Fatalf("lock not auto-released after TTL: %v", err)
	}
	release()
}

func TestLocalCoord_LockCtxCancel(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()
	b.Lock(ctx, "held", 0)
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := b.Lock(ctx2, "held", 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestLocalCoord_Registry(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()
	if err := b.RegistrySet(ctx, "ns", "k1", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	v, err := b.RegistryGet(ctx, "ns", "k1")
	if err != nil || string(v) != "v1" {
		t.Fatalf("get: %v %q", err, v)
	}
	if _, err := b.RegistryGet(ctx, "ns", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	b.RegistrySet(ctx, "ns", "k2", []byte("v2"))
	list, _ := b.RegistryList(ctx, "ns")
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	b.RegistryDelete(ctx, "ns", "k1")
	list, _ = b.RegistryList(ctx, "ns")
	if len(list) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(list))
	}
	// empty value also deletes
	b.RegistrySet(ctx, "ns", "k2", nil)
	list, _ = b.RegistryList(ctx, "ns")
	if len(list) != 0 {
		t.Fatalf("expected 0 after empty-set delete, got %d", len(list))
	}
}

func TestLocalCoord_Queue(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()
	b.QueuePush(ctx, "jobs", []byte("a"))
	b.QueuePush(ctx, "jobs", []byte("b"))
	v, _ := b.QueuePop(ctx, "jobs")
	if string(v) != "a" {
		t.Fatalf("expected a, got %q", v)
	}
	v, _ = b.QueuePop(ctx, "jobs")
	if string(v) != "b" {
		t.Fatalf("expected b, got %q", v)
	}
	// blocking pop unblocks on push
	got := make(chan []byte, 1)
	go func() {
		v, _ := b.QueuePop(ctx, "jobs")
		got <- v
	}()
	time.Sleep(50 * time.Millisecond)
	b.QueuePush(ctx, "jobs", []byte("late"))
	select {
	case v := <-got:
		if string(v) != "late" {
			t.Fatalf("expected late, got %q", v)
		}
	case <-time.After(time.Second):
		t.Fatal("blocking pop did not unblock")
	}
}

func TestLocalCoord_QueuePopCtxCancel(t *testing.T) {
	b := NewLocalBus()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := b.QueuePop(ctx, "empty-queue")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}

func TestLocalCoord_Log(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()
	for _, s := range []string{"x", "y", "z"} {
		b.LogAppend(ctx, "audit", []byte(s))
	}
	entries, next, _ := b.LogRead(ctx, "audit", 0, 2)
	if len(entries) != 2 || string(entries[0]) != "x" || string(entries[1]) != "y" {
		t.Fatalf("read page1: %v", entries)
	}
	if next != 2 {
		t.Fatalf("next cursor = %d", next)
	}
	entries, next, _ = b.LogRead(ctx, "audit", next, 10)
	if len(entries) != 1 || string(entries[0]) != "z" {
		t.Fatalf("read page2: %v", entries)
	}
	if next != 3 {
		t.Fatalf("next cursor = %d", next)
	}
}

func TestLocalCoord_LogReadEmpty(t *testing.T) {
	b := NewLocalBus()
	entries, next, _ := b.LogRead(context.Background(), "nope", 0, 10)
	if len(entries) != 0 || next != 0 {
		t.Fatalf("expected empty, got %v %d", entries, next)
	}
}

func TestAsCoordBus(t *testing.T) {
	local := NewLocalBus()
	if AsCoordBus(local) == nil {
		t.Fatal("LocalBus should satisfy CoordBus")
	}
	if AsCoordBus(nil) != nil {
		t.Fatal("nil bus should yield nil CoordBus")
	}
}

func TestCoordKeys(t *testing.T) {
	if Keys.QueueName("jobs") != "as:queue:jobs" {
		t.Fatal("bad queue key")
	}
	if Keys.LockKey("res") != "as:lock:res" {
		t.Fatal("bad lock key")
	}
	if Keys.ProjectionNS("sess1") != "as:projection:sess1" {
		t.Fatal("bad projection ns")
	}
}

// TestLocalCoord_LockConcurrentSerialised proves mutual exclusion under
// concurrency: a counter incremented under the lock by many goroutines must
// never race.
func TestLocalCoord_LockConcurrentSerialised(t *testing.T) {
	b := NewLocalBus()
	ctx := context.Background()
	var counter int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := b.Lock(ctx, "counter", 0)
			if err != nil {
				t.Errorf("lock: %v", err)
				return
			}
			cur := atomic.LoadInt32(&counter)
			time.Sleep(time.Millisecond) // widen the race window
			atomic.StoreInt32(&counter, cur+1)
			release()
		}()
	}
	wg.Wait()
	if counter != 20 {
		t.Fatalf("counter = %d, want 20 (lock failed to serialise)", counter)
	}
}
