package messagebus

import (
	"context"
	"encoding/json"
	"sync"
	"time"

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

// --- RedisBus TeamBus implementation (cross-process inbox + wakeup) ---

func (b *RedisBus) inboxKey(sid string) string { return b.prefix + ":inbox:" + sid }
func (b *RedisBus) wakeupStreamKey() string    { return b.prefix + ":wakeup:stream" }

func (b *RedisBus) InboxPush(ctx context.Context, sessionID string, msg TeamMessage) error {
	if b.client == nil {
		return ErrClosed
	}
	if msg.SentAt.IsZero() {
		msg.SentAt = time.Now()
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return b.client.RPush(ctx, b.inboxKey(sessionID), data).Err()
}

func (b *RedisBus) InboxDrain(ctx context.Context, sessionID string) ([]TeamMessage, error) {
	if b.client == nil {
		return nil, ErrClosed
	}
	key := b.inboxKey(sessionID)
	pipe := b.client.TxPipeline()
	lrange := pipe.LRange(ctx, key, 0, -1)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	var out []TeamMessage
	for _, raw := range lrange.Val() {
		var m TeamMessage
		if json.Unmarshal([]byte(raw), &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// EnqueueWakeup appends to a Redis Stream. Each SubscribeWakeup consumer reads
// the stream independently from its own cursor, so ALL subscribers receive every
// wakeup (true fan-out). Cross-process duplicate delivery is safe because
// InboxDrain is an atomic LRANGE+DEL: only one process wins the drain, others
// drain an empty inbox and skip. MAXLEN~ trims old entries to bound growth.
func (b *RedisBus) EnqueueWakeup(ctx context.Context, sessionID string) error {
	if b.client == nil {
		return ErrClosed
	}
	return b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.wakeupStreamKey(),
		MaxLen: 10000,
		Approx: true,
		Values: map[string]any{"session": sessionID},
	}).Err()
}

// SubscribeWakeup reads the wakeup stream independently of other consumers
// (true fan-out). It starts from "0" so wakeups enqueued before connecting are
// not lost, then blocks for new entries. Because every consumer sees every
// wakeup, multi-process duplicate handling is made idempotent by the atomic
// InboxDrain. Mirrors Python agentscope's persistent wakeup stream.
func (b *RedisBus) SubscribeWakeup(ctx context.Context) (<-chan WakeupEvent, Cancel, error) {
	if b.client == nil {
		return nil, nil, ErrClosed
	}
	out := make(chan WakeupEvent, defaultSubscriberBuffer)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(out)
		stream := b.wakeupStreamKey()
		lastID := "0" // drain backlog first, then follow new entries
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			default:
			}
			res, err := b.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{stream, lastID},
				Count:   100,
				Block:   5 * time.Second,
			}).Result()
			if err != nil {
				if err == redis.Nil || err == context.Canceled {
					continue
				}
				select {
				case <-done:
					return
				case <-time.After(time.Second):
				}
				continue
			}
			for _, s := range res {
				for _, m := range s.Messages {
					lastID = m.ID
					sid, _ := m.Values["session"].(string)
					if sid == "" {
						continue
					}
					select {
					case out <- WakeupEvent{SessionID: sid}:
					case <-done:
						return
					}
				}
			}
		}
	}()
	cancel := Cancel(func() { once.Do(func() { close(done) }) })
	return out, cancel, nil
}

var _ Bus = (*RedisBus)(nil)
var _ TeamBus = (*RedisBus)(nil)
