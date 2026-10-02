// workspace/pool.go — a warm-up pool for backend workspaces (18.8):
// container-class backends (Docker/K8s/Daytona…) pay a cold-start cost per
// instance; a pool keeps `Preload` instances warm, caps concurrent
// existence at `MaxCreates`, and hands instances out with borrow/return
// semantics so per-session workspace assembly stops paying the cold start.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrPoolClosed is returned by Acquire after Close.
var ErrPoolClosed = errors.New("workspace pool: closed")

// ErrPoolDrainTimeout is returned by Close when borrowed instances did not
// all come back within the drain timeout (they are force-closed anyway).
var ErrPoolDrainTimeout = errors.New("workspace pool: drain timeout with instances still borrowed")

// PoolFactory creates one backend workspace instance. Implementations wrap
// the backend's own constructor (e.g. docker run + wait-ready).
type PoolFactory func(ctx context.Context) (Workspace, error)

// PoolConfig sizes the pool. Zero Preload means no warm instances (pure
// concurrency capping); zero MaxCreates means uncapped (Preload still holds).
type PoolConfig struct {
	// Preload is how many warm instances the pool keeps idle.
	Preload int
	// MaxCreates caps how many instances may exist at once (borrowed +
	// idle). Extra Acquires wait for a release instead of over-creating.
	MaxCreates int
}

// Pool is a borrowing pool over a backend factory. Safe for concurrent use.
type Pool struct {
	factory PoolFactory
	cfg     PoolConfig

	mu       sync.Mutex
	idle     []Workspace
	living   int // borrowed + idle (the concurrency cap counts these)
	borrowed int
	closed   bool
	// slot signals waiting Acquires that a slot or idle instance appeared.
	slot *sync.Cond

	// wg tracks in-flight creations + borrows so Close can drain.
	wg sync.WaitGroup
}

// NewPool builds a pool and eagerly warms it to cfg.Preload instances
// (failures are tolerated: the refill loop retries; a fully failing backend
// surfaces its error on Acquire instead of at assembly — principle 7).
func NewPool(ctx context.Context, factory PoolFactory, cfg PoolConfig) *Pool {
	p := &Pool{factory: factory, cfg: cfg}
	p.slot = sync.NewCond(&p.mu)
	go p.refill(ctx)
	return p
}

// Acquire borrows a workspace: an idle instance if one is warm, else a fresh
// create while under MaxCreates, else it waits (cancellable) for a slot.
func (p *Pool) Acquire(ctx context.Context) (Workspace, error) {
	p.mu.Lock()
	for {
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}
		if n := len(p.idle); n > 0 {
			ws := p.idle[n-1]
			p.idle = p.idle[:n-1]
			p.borrowed++
			p.mu.Unlock()
			p.wg.Add(1)
			// Keep the pool topped up as it drains.
			go p.refill(context.Background())
			return ws, nil
		}
		if p.cfg.MaxCreates <= 0 || p.living < p.cfg.MaxCreates {
			p.living++
			p.borrowed++
			p.mu.Unlock()
			p.wg.Add(1)
			ws, err := p.factory(ctx)
			if err != nil {
				// Creation failed: the slot frees immediately (the failed
				// instance never existed).
				p.mu.Lock()
				p.living--
				p.borrowed--
				p.mu.Unlock()
				p.wg.Done()
				p.slot.Signal()
				return nil, fmt.Errorf("workspace pool: create: %w", err)
			}
			return ws, nil
		}
		// At the cap: wait for a release/discard/close.
		if err := ctx.Err(); err != nil {
			p.mu.Unlock()
			return nil, err
		}
		waitDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				p.slot.Broadcast() // wake so this waiter can observe ctx.Err
			case <-waitDone:
			}
		}()
		p.slot.Wait()
		close(waitDone)
	}
}

// Release returns a healthy workspace to the pool. The instance goes back to
// the idle set so a waiting Acquire (or the next one) reuses it instead of
// paying another cold start — Preload is the refill floor, not the idle
// ceiling; the idle set is naturally bounded by MaxCreates. Only a closed
// pool closes the instance here.
func (p *Pool) Release(ws Workspace) {
	if ws == nil {
		return
	}
	p.mu.Lock()
	p.borrowed--
	keep := !p.closed
	if keep {
		p.idle = append(p.idle, ws)
	} else {
		p.living--
	}
	p.mu.Unlock()
	p.wg.Done()
	if !keep {
		_ = ws.Close()
	}
	p.slot.Signal()
}

// Discard destroys a workspace that is no longer trusted (broken
// execution, dropped connection): it is closed and its slot freed, so the
// cap opens up and the refill loop replaces it.
func (p *Pool) Discard(ws Workspace) {
	if ws == nil {
		return
	}
	p.mu.Lock()
	p.borrowed--
	p.living--
	p.mu.Unlock()
	p.wg.Done()
	_ = ws.Close()
	p.slot.Signal()
	go p.refill(context.Background())
}

// Close stops the pool: no new borrows; idle instances are closed
// immediately; borrowed instances are waited for up to drainTimeout (they
// are then force-closed) and the refill loop is halted.
func (p *Pool) Close(drainTimeout context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	idle := p.idle
	p.idle = nil
	stillBorrowed := p.borrowed
	p.mu.Unlock()

	p.slot.Broadcast()

	for _, ws := range idle {
		_ = ws.Close()
		// Idle instances leaving the pool free their living slots.
		p.mu.Lock()
		p.living--
		p.mu.Unlock()
	}
	if stillBorrowed == 0 {
		return nil
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-drainTimeout.Done():
		// Borrowed instances never came back: they are the borrowers'
		// responsibility now; report the drain failure honestly.
		return ErrPoolDrainTimeout
	}
}

// Stats reports pool occupancy (for metrics and tests).
func (p *Pool) Stats() (idle, borrowed, living int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idle), p.borrowed, p.living
}

// refill tops the pool back up to the Preload watermark. The watermark
// counts LIVING instances (idle + borrowed): a borrower that will release
// its instance must not trigger a duplicate warm-up — that would double the
// population on every borrow. Only real losses (discards, failed creates,
// consumption below the watermark) create replacements. Invoked at
// construction (warm-up) and after borrows/releases/discards.
func (p *Pool) refill(ctx context.Context) {
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		need := p.cfg.Preload - p.living
		if need <= 0 {
			p.mu.Unlock()
			return
		}
		if p.cfg.MaxCreates > 0 && p.living >= p.cfg.MaxCreates {
			p.mu.Unlock()
			return // the cap governs; Acquire's create path will top up later
		}
		p.living++
		p.mu.Unlock()

		ws, err := p.factory(ctx)
		if ctx.Err() != nil {
			// Pool is being torn down: drop the slot and stop.
			if ws != nil {
				_ = ws.Close()
			}
			p.mu.Lock()
			p.living--
			p.mu.Unlock()
			return
		}
		p.mu.Lock()
		if err != nil {
			// Backend warming failed: free the slot; the next Acquire (or a
			// later borrow-triggered refill) retries.
			p.living--
			p.mu.Unlock()
			return
		}
		if p.closed {
			p.mu.Unlock()
			_ = ws.Close()
			p.mu.Lock()
			p.living--
			p.mu.Unlock()
			return
		}
		p.idle = append(p.idle, ws)
		p.mu.Unlock()
		p.slot.Signal()
	}
}
