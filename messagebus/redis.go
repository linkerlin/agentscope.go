package messagebus

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisBus implements Bus using Redis pub/sub, enabling multiprocess and
// distributed deployments (mirrors Python agentscope's redis message bus from
// #1849). A single Redis instance coordinates publishers and subscribers across
// many gateway processes.
type RedisBus struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisBus creates a Redis-backed bus. prefix namespaces the pub/sub channels
// (defaults to "as:bus") so multiple busses can share one Redis instance.
func NewRedisBus(client redis.UniversalClient, prefix string) *RedisBus {
	if prefix == "" {
		prefix = "as:bus"
	}
	return &RedisBus{client: client, prefix: prefix}
}

func (b *RedisBus) chanKey(channel string) string { return b.prefix + ":" + channel }

// Publish broadcasts payload to all subscribers of channel across every process
// connected to the same Redis instance.
func (b *RedisBus) Publish(ctx context.Context, channel string, payload []byte) error {
	if b.client == nil {
		return ErrClosed
	}
	return b.client.Publish(ctx, b.chanKey(channel), payload).Err()
}

// Subscribe listens on the given Redis channels and forwards messages to the
// returned channel. The cancel func tears down the subscription (and the
// goroutine). The returned channel is closed on cancel, context cancellation,
// or if the Redis subscription stream ends.
func (b *RedisBus) Subscribe(ctx context.Context, channels ...string) (<-chan Message, Cancel, error) {
	if b.client == nil {
		return nil, nil, ErrClosed
	}
	if len(channels) == 0 {
		return nil, nil, nil
	}
	keys := make([]string, len(channels))
	for i, c := range channels {
		keys[i] = b.chanKey(c)
	}
	pubsub := b.client.Subscribe(ctx, keys...)
	out := make(chan Message, defaultSubscriberBuffer)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(out)
		defer func() { _ = pubsub.Close() }()
		msgCh := pubsub.Channel()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				select {
				case out <- Message{Channel: msg.Channel, Payload: []byte(msg.Payload)}:
				case <-done:
					return
				}
			}
		}
	}()
	cancel := Cancel(func() {
		once.Do(func() { close(done) })
	})
	return out, cancel, nil
}

// Close is a no-op for RedisBus: subscriptions manage their own lifetime via
// their cancel funcs / context, and the shared Redis client is owned by the
// caller.
func (b *RedisBus) Close() error { return nil }

var _ Bus = (*RedisBus)(nil)
