// Package async provides a minimal asynchronous task execution pool for
// background agent operations (e.g., long-running tool calls, batch
// embedding generation, state snapshots).
package async

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Status represents the lifecycle state of a task.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Task holds the result and metadata of a submitted unit of work.
type Task struct {
	ID      string
	Status  Status
	Result  any
	Error   error
	Created time.Time
	Started time.Time
	Ended   time.Time
}

// Pool is a fixed-size goroutine pool that executes submitted functions
// asynchronously. It provides status tracking and graceful shutdown.
type Pool struct {
	workers int
	queue   chan *workItem
	tasks   map[string]*Task
	mu      sync.RWMutex
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
	// seq generates monotonically-unique task IDs, avoiding collisions when
	// multiple tasks are submitted within the same clock tick (which could
	// happen with a pure time.Now().UnixNano() id under load).
	seq int64
}

type workItem struct {
	id string
	fn func() (any, error)
}

// NewPool creates a pool with the given number of workers and queue capacity.
func NewPool(workers, queueCap int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers: workers,
		queue:   make(chan *workItem, queueCap),
		tasks:   make(map[string]*Task),
		ctx:     ctx,
		cancel:  cancel,
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// Submit enqueues a function for asynchronous execution and returns a task ID.
func (p *Pool) Submit(fn func() (any, error)) string {
	// Monotonic counter guarantees uniqueness even under rapid concurrent
	// submission; the timestamp is kept for human-readability/debugging.
	id := fmt.Sprintf("task_%d_%d", atomic.AddInt64(&p.seq, 1), time.Now().UnixNano())
	task := &Task{
		ID:      id,
		Status:  StatusPending,
		Created: time.Now(),
	}
	p.mu.Lock()
	p.tasks[id] = task
	closed := p.closed
	p.mu.Unlock()

	if closed {
		p.mu.Lock()
		task.Status = StatusCancelled
		task.Ended = time.Now()
		p.mu.Unlock()
		return id
	}

	select {
	case p.queue <- &workItem{id: id, fn: fn}:
	case <-p.ctx.Done():
		p.mu.Lock()
		task.Status = StatusCancelled
		task.Ended = time.Now()
		p.mu.Unlock()
	}
	return id
}

// List returns snapshots of all tracked tasks.
func (p *Pool) List() []*Task {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*Task, 0, len(p.tasks))
	for _, t := range p.tasks {
		cpy := *t
		out = append(out, &cpy)
	}
	return out
}

// Cancel marks a pending task as cancelled.
func (p *Pool) Cancel(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ok := p.tasks[id]
	if !ok || t.Status != StatusPending {
		return false
	}
	t.Status = StatusCancelled
	t.Ended = time.Now()
	return true
}

func (p *Pool) Task(id string) (*Task, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	t, ok := p.tasks[id]
	if !ok {
		return nil, false
	}
	// Return a copy to avoid races.
	cpy := *t
	return &cpy, true
}

// Status is a convenience helper that returns the status of a task.
func (p *Pool) Status(id string) Status {
	t, _ := p.Task(id)
	if t == nil {
		return ""
	}
	return t.Status
}

// Shutdown signals the pool to stop accepting new work and waits for
// in-flight tasks to complete or the context to be cancelled.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.queue)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		p.cancel()
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case wi, ok := <-p.queue:
			if !ok {
				return
			}
			p.mu.Lock()
			task := p.tasks[wi.id]
			if task == nil {
				p.mu.Unlock()
				continue
			}
			task.Status = StatusRunning
			task.Started = time.Now()
			p.mu.Unlock()

			res, err := wi.fn()

			p.mu.Lock()
			task = p.tasks[wi.id]
			if task != nil {
				if err != nil {
					task.Error = err
					task.Status = StatusFailed
				} else {
					task.Result = res
					task.Status = StatusCompleted
				}
				task.Ended = time.Now()
			}
			p.mu.Unlock()
		}
	}
}
