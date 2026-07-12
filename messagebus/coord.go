package messagebus

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CoordBus extends a Bus with domain-agnostic coordination primitives: a
// distributed lock, a key-value registry, a FIFO queue, and an append-log.
// These are the building blocks for cross-process coordination (Python
// agentscope's app/message_bus generic primitives). Backends: LocalBus
// (in-process) and RedisBus (cross-process).
//
// Like TeamBus, CoordBus is an OPTIONAL interface — obtain it via AsCoordBus.
type CoordBus interface {
	// Lock acquires a named lock held for ttl (zero = until release is called
	// or the process exits). Returns a Cancel that releases the lock. If the
	// lock cannot be acquired before ctx is done, returns ctx.Err().
	Lock(ctx context.Context, key string, ttl time.Duration) (Cancel, error)
	// RegistrySet stores value under ns/key. An empty value deletes the entry.
	RegistrySet(ctx context.Context, ns, key string, value []byte) error
	// RegistryGet reads the value for ns/key. Returns ErrNotFound if absent.
	RegistryGet(ctx context.Context, ns, key string) ([]byte, error)
	// RegistryList returns all key/value pairs under ns.
	RegistryList(ctx context.Context, ns string) (map[string][]byte, error)
	// RegistryDelete removes ns/key.
	RegistryDelete(ctx context.Context, ns, key string) error
	// QueuePush appends value to the named FIFO queue.
	QueuePush(ctx context.Context, name string, value []byte) error
	// QueuePop removes and returns the head of the named queue, blocking until
	// a value is available or ctx is done.
	QueuePop(ctx context.Context, name string) ([]byte, error)
	// LogAppend appends value to the named log and returns its 0-based index.
	LogAppend(ctx context.Context, ns string, value []byte) (int64, error)
	// LogRead returns up to limit entries starting at cursor (0-based index),
	// plus the next cursor (== cursor+len(entries)). Use limit<=0 for a
	// reasonable default.
	LogRead(ctx context.Context, ns string, cursor int64, limit int) ([][]byte, int64, error)
}

// ErrNotFound is returned by registry lookups for missing keys.
var ErrNotFound = errors.New("messagebus: key not found")

// AsCoordBus returns a CoordBus view of b if it implements CoordBus, else nil.
func AsCoordBus(b Bus) CoordBus {
	if cb, ok := b.(CoordBus); ok {
		return cb
	}
	return nil
}

// --- LocalBus CoordBus implementation ---

// localLock is a capacity-1 channel used as a binary semaphore.
type localLock struct {
	ch chan struct{}
}

// localQueue is an unbounded FIFO with a ctx-cancellable blocking pop.
type localQueue struct {
	mu     sync.Mutex
	items  [][]byte
	notify chan struct{}
}

func newLocalQueue() *localQueue {
	return &localQueue{notify: make(chan struct{}, 1)}
}

func (q *localQueue) push(v []byte) {
	q.mu.Lock()
	q.items = append(q.items, v)
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *localQueue) pop(ctx context.Context) ([]byte, error) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			v := q.items[0]
			q.items = q.items[1:]
			q.mu.Unlock()
			return v, nil
		}
		q.mu.Unlock()
		select {
		case <-q.notify:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (b *LocalBus) Lock(ctx context.Context, key string, ttl time.Duration) (Cancel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrClosed
	}
	lk, ok := b.locks[key]
	if !ok {
		lk = &localLock{ch: make(chan struct{}, 1)}
		b.locks[key] = lk
	}
	b.mu.Unlock()

	select {
	case lk.ch <- struct{}{}:
		// acquired
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var once sync.Once
	var timer *time.Timer
	if ttl > 0 {
		timer = time.AfterFunc(ttl, func() {
			select {
			case <-lk.ch:
			default:
			}
		})
	}
	return Cancel(func() {
		once.Do(func() {
			if timer != nil {
				timer.Stop()
			}
			select {
			case <-lk.ch:
			default:
			}
		})
	}), nil
}

func (b *LocalBus) RegistrySet(ctx context.Context, ns, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	if len(value) == 0 {
		if m, ok := b.registry[ns]; ok {
			delete(m, key)
			if len(m) == 0 {
				delete(b.registry, ns)
			}
		}
		return nil
	}
	if b.registry[ns] == nil {
		b.registry[ns] = map[string][]byte{}
	}
	b.registry[ns][key] = append([]byte(nil), value...)
	return nil
}

func (b *LocalBus) RegistryGet(ctx context.Context, ns, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, ErrClosed
	}
	m, ok := b.registry[ns]
	if !ok {
		return nil, ErrNotFound
	}
	v, ok := m[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (b *LocalBus) RegistryList(ctx context.Context, ns string) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, ErrClosed
	}
	out := make(map[string][]byte, len(b.registry[ns]))
	for k, v := range b.registry[ns] {
		out[k] = append([]byte(nil), v...)
	}
	return out, nil
}

func (b *LocalBus) RegistryDelete(ctx context.Context, ns, key string) error {
	return b.RegistrySet(ctx, ns, key, nil)
}

func (b *LocalBus) QueuePush(ctx context.Context, name string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrClosed
	}
	q, ok := b.queues[name]
	if !ok {
		q = newLocalQueue()
		b.queues[name] = q
	}
	b.mu.Unlock()
	q.push(value)
	return nil
}

func (b *LocalBus) QueuePop(ctx context.Context, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrClosed
	}
	q, ok := b.queues[name]
	b.mu.Unlock()
	if !ok {
		q = newLocalQueue() // create on demand so pop can block for future pushes
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return nil, ErrClosed
		}
		if existing, ok2 := b.queues[name]; ok2 {
			q = existing
		} else {
			b.queues[name] = q
		}
		b.mu.Unlock()
	}
	return q.pop(ctx)
}

func (b *LocalBus) LogAppend(ctx context.Context, ns string, value []byte) (int64, error) {
	if err := ctx.Err(); err != nil {
		return -1, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return -1, ErrClosed
	}
	idx := int64(len(b.logs[ns]))
	b.logs[ns] = append(b.logs[ns], append([]byte(nil), value...))
	return idx, nil
}

func (b *LocalBus) LogRead(ctx context.Context, ns string, cursor int64, limit int) ([][]byte, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, cursor, err
	}
	if limit <= 0 {
		limit = 100
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, cursor, ErrClosed
	}
	entries := b.logs[ns]
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= int64(len(entries)) {
		return nil, cursor, nil
	}
	end := cursor + int64(limit)
	if end > int64(len(entries)) {
		end = int64(len(entries))
	}
	out := make([][]byte, 0, end-cursor)
	for i := cursor; i < end; i++ {
		out = append(out, append([]byte(nil), entries[i]...))
	}
	return out, end, nil
}

var _ CoordBus = (*LocalBus)(nil)
