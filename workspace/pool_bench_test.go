package workspace

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// BenchColdStart* (18.8 acceptance): a warm pool hands out instances in
// microseconds where the raw factory pays the backend cold start every
// time. The fake factory models a 5ms container boot — the shape, not the
// absolute number, is what the benchmark locks.
const benchColdStart = 5 * time.Millisecond

func benchFactory() *countingFactory { return &countingFactory{createDelay: benchColdStart} }

func BenchmarkColdStart_NoPool(b *testing.B) {
	f := benchFactory()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws, err := f.new(ctx)
		if err != nil {
			b.Fatal(err)
		}
		_ = ws.Close()
	}
}

func BenchmarkColdStart_WarmPool(b *testing.B) {
	f := benchFactory()
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 4, MaxCreates: 4})
	defer p.Close(context.Background())
	waitIdleB(b, p, 4)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws, err := p.Acquire(ctx)
		if err != nil {
			b.Fatal(err)
		}
		p.Release(ws)
	}
}

// BenchmarkColdStart_ConcurrentCap proves the cap under load: with
// MaxCreates=2 the pool serves 8 concurrent borrowers through exactly 2
// instances (creations stay at 2), versus the uncapped factory creating 8.
func BenchmarkColdStart_ConcurrentCap(b *testing.B) {
	f := benchFactory()
	p := NewPool(context.Background(), f.new, PoolConfig{Preload: 0, MaxCreates: 2})
	defer p.Close(context.Background())
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var done atomic.Int32
		for w := 0; w < 8; w++ {
			go func() {
				ws, err := p.Acquire(ctx)
				if err != nil {
					b.Error(err)
					return
				}
				p.Release(ws)
				done.Add(1)
			}()
		}
		for done.Load() < 8 {
			time.Sleep(time.Millisecond)
		}
	}
	f.mu.Lock()
	total := f.total
	f.mu.Unlock()
	if total > 2 {
		b.Fatalf("cap violated under benchmark load: creations=%d", total)
	}
}

func waitIdleB(b *testing.B, p *Pool, want int) {
	b.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if idle, _, _ := p.Stats(); idle >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	b.Fatal("pool never warmed up for benchmark")
}
