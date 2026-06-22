// Package messagebus provides a pub/sub message bus abstraction for
// cross-component and cross-process coordination. It mirrors Python
// agentscope's app/message_bus (refactored in #1849 for multiprocess /
// distributed deployment).
//
// Two implementations are provided:
//   - LocalBus: in-process fan-out (default, zero dependencies)
//   - RedisBus: Redis pub/sub for multiprocess / distributed deployment
package messagebus

import (
	"context"
	"errors"
	"sync"
)

// defaultSubscriberBuffer sizes the per-subscriber buffered channel. A slow
// subscriber never blocks the publisher for LocalBus (drops), and avoids
// unbounded memory growth for RedisBus.
const defaultSubscriberBuffer = 64

// ErrClosed is returned when operating on a closed bus.
var ErrClosed = errors.New("messagebus: bus is closed")

// Message is a single published event.
type Message struct {
	// Channel is the topic the message was published to.
	Channel string
	// Payload is the opaque message bytes (callers encode JSON/protobuf/etc).
	Payload []byte
}

// Bus is the pub/sub abstraction. Implementations must be safe for concurrent
// use by multiple publishers and subscribers.
type Bus interface {
	// Publish broadcasts payload to all subscribers of channel.
	Publish(ctx context.Context, channel string, payload []byte) error
	// Subscribe returns a channel receiving messages published to any of the
	// given channels, plus a cancel func that tears down the subscription.
	// The returned channel is closed when cancel is invoked or the bus closes.
	Subscribe(ctx context.Context, channels ...string) (<-chan Message, Cancel, error)
	// Close releases all bus-wide resources and closes all subscriber channels.
	Close() error
}

// Cancel tears down a subscription. Safe to call multiple times.
type Cancel func()

// LocalBus is an in-process pub/sub bus. Publish is non-blocking: messages are
// dropped (not delivered) when a subscriber's buffer is full, so a slow
// subscriber never stalls the publisher or other subscribers.
type LocalBus struct {
	mu       sync.RWMutex
	subs     map[string]map[int]chan Message
	nextID   int
	closed   bool
	inbox    map[string][]TeamMessage
	wakeSubs map[int]chan WakeupEvent
	wakeNext int
}

// NewLocalBus creates an empty in-process bus.
func NewLocalBus() *LocalBus {
	return &LocalBus{
		subs:     map[string]map[int]chan Message{},
		inbox:    map[string][]TeamMessage{},
		wakeSubs: map[int]chan WakeupEvent{},
	}
}

// Publish fans payload out to all current subscribers of channel.
func (b *LocalBus) Publish(ctx context.Context, channel string, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return ErrClosed
	}
	for _, ch := range b.subs[channel] {
		select {
		case ch <- Message{Channel: channel, Payload: payload}:
		default:
			// Drop on full buffer; never block the publisher.
		}
	}
	return nil
}

// Subscribe registers a new subscription on the given channels.
func (b *LocalBus) Subscribe(ctx context.Context, channels ...string) (<-chan Message, Cancel, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, ErrClosed
	}
	b.nextID++
	id := b.nextID
	ch := make(chan Message, defaultSubscriberBuffer)
	for _, c := range channels {
		if b.subs[c] == nil {
			b.subs[c] = map[int]chan Message{}
		}
		b.subs[c][id] = ch
	}
	var once sync.Once
	cancel := Cancel(func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			for _, c := range channels {
				if subs, ok := b.subs[c]; ok {
					delete(subs, id)
					if len(subs) == 0 {
						delete(b.subs, c)
					}
				}
			}
			close(ch)
		})
	})
	return ch, cancel, nil
}

// Close shuts the bus down: all subscriber channels are closed and further
// Publish/Subscribe calls return ErrClosed.
func (b *LocalBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, subs := range b.subs {
		for _, ch := range subs {
			close(ch)
		}
	}
	b.subs = map[string]map[int]chan Message{}
	for _, ch := range b.wakeSubs {
		close(ch)
	}
	b.wakeSubs = map[int]chan WakeupEvent{}
	b.inbox = map[string][]TeamMessage{}
	return nil
}
