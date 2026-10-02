package workspace

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"io/fs"
)

// fakeWS is a trivial Workspace whose Close is observable.
type fakeWS struct {
	id     int
	closed atomic.Bool
}

func (f *fakeWS) ID() string   { return "fake" }
func (f *fakeWS) Type() string { return "fake" }
func (f *fakeWS) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, errors.New("unimplemented")
}
func (f *fakeWS) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	return errors.New("unimplemented")
}
func (f *fakeWS) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	return nil, errors.New("unimplemented")
}
func (f *fakeWS) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return errors.New("unimplemented")
}
func (f *fakeWS) Stat(ctx context.Context, path string) (FileInfo, error) {
	return FileInfo{}, errors.New("unimplemented")
}
func (f *fakeWS) Execute(ctx context.Context, command string, opts ExecuteOptions) (*ExecuteResult, error) {
	return &ExecuteResult{ExitCode: 0}, nil
}
func (f *fakeWS) Close() error { f.closed.Store(true); return nil }

// countingFactory builds fakeWS instances, tracking concurrent creations and
// total creates (for cap and cold-start assertions).
type countingFactory struct {
	createDelay time.Duration
	failFirst   atomic.Int32 // first N creations fail (failure-recycle path)

	mu       sync.Mutex
	creating int
	peak     int
	total    int
	nextID   int
}

func (f *countingFactory) new(ctx context.Context) (Workspace, error) {
	f.mu.Lock()
	f.creating++
	if f.creating > f.peak {
		f.peak = f.creating
	}
	f.total++
	f.nextID++
	id := f.nextID
	f.mu.Unlock()

	if f.createDelay > 0 {
		select {
		case <-time.After(f.createDelay):
		case <-ctx.Done():
			f.mu.Lock()
			f.creating--
			f.mu.Unlock()
			return nil, ctx.Err()
		}
	}
	for {
		n := f.failFirst.Load()
		if n <= 0 {
			break
		}
		if f.failFirst.CompareAndSwap(n, n-1) {
			f.mu.Lock()
			f.creating--
			f.mu.Unlock()
			return nil, errors.New("backend cold start failed")
		}
	}

	f.mu.Lock()
	f.creating--
	f.mu.Unlock()
	return &fakeWS{id: id}, nil
}

// waitIdle polls until the pool reports the wanted idle count (warm-up is
// asynchronous).
func waitIdle(t *testing.T, p *Pool, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if idle, _, _ := p.Stats(); idle >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	idle, borrowed, living := p.Stats()
	t.Fatalf("pool never warmed to %d idle (idle=%d borrowed=%d living=%d)", want, idle, borrowed, living)
}

// TestPool_WarmBorrowReturn: the pool preloads, lends warm instances, and
// returned healthy instances go back to the idle set without re-creating.
func TestPool_WarmBorrowReturn(t *testing.T) {
	f := &countingFactory{}
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 2, MaxCreates: 4})
	waitIdle(t, p, 2)

	ws1, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ws2, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if idle, borrowed, _ := p.Stats(); idle != 0 || borrowed != 2 {
		t.Fatalf("after borrows: idle=%d borrowed=%d", idle, borrowed)
	}
	p.Release(ws1)
	p.Release(ws2)
	waitIdle(t, p, 2)

	f.mu.Lock()
	created := f.total
	f.mu.Unlock()
	if created != 2 {
		t.Fatalf("releases must reuse instances, creations=%d", created)
	}
}

// TestPool_ConcurrencyCap: with MaxCreates=N, concurrent acquires beyond N
// wait for releases instead of over-creating (peak concurrent existence is
// the cap), and every waiter still gets an instance.
func TestPool_ConcurrencyCap(t *testing.T) {
	f := &countingFactory{createDelay: 5 * time.Millisecond}
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 0, MaxCreates: 2})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws, err := p.Acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			time.Sleep(10 * time.Millisecond) // hold the slot
			p.Release(ws)
		}()
	}
	wg.Wait()

	f.mu.Lock()
	peak, total := f.peak, f.total
	f.mu.Unlock()
	if peak > 2 {
		t.Fatalf("concurrency cap violated: peak concurrent creations=%d", peak)
	}
	if total > 2 {
		t.Fatalf("cap must reuse instances across waiters, creations=%d", total)
	}
}

// TestPool_FailedCreateFreesSlot: a failing factory surfaces the error on
// Acquire and frees the slot (the next acquire retries and can succeed).
func TestPool_FailedCreateFreesSlot(t *testing.T) {
	f := &countingFactory{}
	f.failFirst.Store(2) // first two creations fail
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 0, MaxCreates: 1})

	if _, err := p.Acquire(context.Background()); err == nil {
		t.Fatal("first acquire must fail")
	}
	if _, err := p.Acquire(context.Background()); err == nil {
		t.Fatal("second acquire must fail")
	}
	// The failed creates must not have leaked the only slot.
	ws, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("third acquire must succeed after failures, got %v", err)
	}
	p.Release(ws)
}

// TestPool_DiscardRecycles: discarding a broken instance closes it, frees
// the slot, and the refill loop replaces it.
func TestPool_DiscardRecycles(t *testing.T) {
	f := &countingFactory{}
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 1, MaxCreates: 1})
	waitIdle(t, p, 1)

	ws, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	broken := ws.(*fakeWS)
	p.Discard(broken)
	if !broken.closed.Load() {
		t.Fatal("discarded instance must be closed")
	}
	waitIdle(t, p, 1) // refilled

	f.mu.Lock()
	total := f.total
	f.mu.Unlock()
	if total != 2 {
		t.Fatalf("discard must trigger a replacement, creations=%d", total)
	}
}

// TestPool_CloseDrains: Close closes idle instances immediately and waits
// for borrowed ones; a drain timeout reports ErrPoolDrainTimeout while the
// instances remain the borrower's responsibility.
func TestPool_CloseDrains(t *testing.T) {
	f := &countingFactory{}
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 2, MaxCreates: 4})
	waitIdle(t, p, 2)

	ws, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Drain with the borrower still holding one: timeout surfaces, idle
	// instances are closed.
	drainCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := p.Close(drainCtx); !errors.Is(err, ErrPoolDrainTimeout) {
		t.Fatalf("expected drain timeout, got %v", err)
	}
	idle, _, _ := p.Stats()
	if idle != 0 {
		t.Fatalf("idle must be closed on Close, got %d", idle)
	}
	if _, err := p.Acquire(context.Background()); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("acquire after close must fail, got %v", err)
	}

	// The borrower returns it afterwards: the late release closes it.
	p.Release(ws)
	if !ws.(*fakeWS).closed.Load() {
		t.Fatal("late release after Close must close the instance")
	}

	// A clean Close (nothing borrowed) returns nil.
	p2 := NewPool(context.Background(), (&countingFactory{}).new, PoolConfig{Preload: 1})
	waitIdle(t, p2, 1)
	if err := p2.Close(context.Background()); err != nil {
		t.Fatalf("clean close: %v", err)
	}
}

// TestPool_AcquireCtxCancel: waiting at the cap honours context
// cancellation.
func TestPool_AcquireCtxCancel(t *testing.T) {
	f := &countingFactory{}
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 0, MaxCreates: 1})
	defer p.Close(context.Background())

	ws, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release(ws)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := p.Acquire(ctx); err == nil {
		t.Fatal("capped acquire must fail on ctx cancel")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancel must be prompt, took %v", elapsed)
	}
}
