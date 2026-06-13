package a2a

import (
	"context"
	"sync"
)

// ChangeOp describes the kind of registry change.
type ChangeOp string

const (
	// ChangeOpRegister is emitted when a new agent is registered or discovered.
	ChangeOpRegister ChangeOp = "register"
	// ChangeOpRemove is emitted when an agent is unregistered.
	ChangeOpRemove ChangeOp = "remove"
	// ChangeOpHealth is emitted when an agent's health status changes.
	ChangeOpHealth ChangeOp = "health"
)

// RegistryChange notifies watchers about a change in the registry.
type RegistryChange struct {
	URL     string   `json:"url"`
	Healthy bool     `json:"healthy"`
	Op      ChangeOp `json:"op"`
}

// watcherSet manages a set of buffered channels that receive registry changes.
type watcherSet struct {
	mu       sync.Mutex
	watchers []chan RegistryChange
}

func (w *watcherSet) add(ch chan RegistryChange) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.watchers = append(w.watchers, ch)
}

func (w *watcherSet) remove(ch chan RegistryChange) {
	w.mu.Lock()
	defer w.mu.Unlock()
	filtered := w.watchers[:0]
	for _, c := range w.watchers {
		if c != ch {
			filtered = append(filtered, c)
		}
	}
	w.watchers = filtered
}

func (w *watcherSet) notify(change RegistryChange) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, ch := range w.watchers {
		select {
		case ch <- change:
		default:
			// Drop if the watcher is slow to avoid blocking producers.
		}
	}
}

// Watch returns a channel that receives RegistryChange events. The channel is
// closed when ctx is cancelled. Callers must drain or stop reading from the
// channel to avoid leaking the internal goroutine.
func (r *Registry) Watch(ctx context.Context) <-chan RegistryChange {
	ch := make(chan RegistryChange, 16)
	r.watchers.add(ch)
	go func() {
		<-ctx.Done()
		r.watchers.remove(ch)
		close(ch)
	}()
	return ch
}
