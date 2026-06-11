package event

import (
	"context"
	"strconv"
	"sync"
)

// Bus is a publish/subscribe event bus for streaming AgentEvents to multiple
// consumers (e.g. Studio UI, loggers, monitors).
// It is safe for concurrent use.
type Bus struct {
	mu       sync.RWMutex
	subs     map[string]*subscription
	nextID   int
	capacity int
}

type subscription struct {
	id   string
	ch   chan AgentEvent
	done chan struct{}
}

// NewBus creates an event bus with a per-subscriber channel buffer size.
func NewBus(bufferSize int) *Bus {
	if bufferSize < 1 {
		bufferSize = 64
	}
	return &Bus{subs: make(map[string]*subscription), capacity: bufferSize}
}

// Subscribe registers a new subscriber and returns a receive-only channel plus a done
// channel that is closed by Unsubscribe. The caller must call Unsubscribe(id) when done.
// Receivers can select on done to exit cleanly without bus closing the event ch.
func (b *Bus) Subscribe() (id string, ch <-chan AgentEvent, done <-chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id = fmtSubID(b.nextID)
	sub := &subscription{
		id:   id,
		ch:   make(chan AgentEvent, b.capacity),
		done: make(chan struct{}),
	}
	b.subs[id] = sub
	return id, sub.ch, sub.done
}

// Unsubscribe removes a subscriber and closes its done channel (ch is left open for GC;
// no close(ch) to avoid any send-on-closed race with concurrent Publish).
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	sub, ok := b.subs[id]
	delete(b.subs, id)
	b.mu.Unlock()
	if ok {
		close(sub.done)
		// Intentionally not closing sub.ch to eliminate close/send data race window.
	}
}

// Publish broadcasts an event to all active subscribers.
// Non-blocking: if a subscriber's buffer is full, the event is dropped for that subscriber.
func (b *Bus) Publish(ev AgentEvent) {
	b.mu.RLock()
	subs := make([]*subscription, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		case <-sub.done:
			// subscriber unsubbed
		default:
			// Drop event for slow consumer to avoid blocking the bus.
		}
	}
}

// PublishSync broadcasts an event to all subscribers, blocking until each
// subscriber has either received the event or is full.
func (b *Bus) PublishSync(ev AgentEvent) {
	b.mu.RLock()
	subs := make([]*subscription, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		case <-sub.done:
		}
	}
}

// SubscriberCount returns the number of active subscribers.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// Forward copies every event from src to the bus until src is closed or ctx is done.
func (b *Bus) Forward(ctx context.Context, src <-chan AgentEvent) {
	for {
		select {
		case ev, ok := <-src:
			if !ok {
				return
			}
			b.Publish(ev)
		case <-ctx.Done():
			return
		}
	}
}

func fmtSubID(n int) string {
	return "sub-" + strconv.Itoa(n)
}
