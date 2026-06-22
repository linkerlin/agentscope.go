package messagebus

import (
	"context"
	"sync"
	"time"
)

// TeamMessage is a single message pushed to a worker session's inbox. Mirrors
// Python agentscope's <team-message> HintBlock delivered via message_bus.inbox_push.
type TeamMessage struct {
	From    string    `json:"from"`    // sender display name
	Content string    `json:"content"` // message body
	SentAt  time.Time `json:"sent_at"`
}

// WakeupEvent signals that a session should be re-run to drain its inbox.
type WakeupEvent struct {
	SessionID string `json:"session_id"`
}

// TeamBus extends Bus with agent-team collaboration primitives: a per-session
// inbox queue (single-consumer, drain-clears) and a wakeup signal stream that
// drives the WakeupDispatcher to re-run idle sessions. Mirrors Python
// agentscope's message_bus inbox_push / enqueue_wakeup / subscribe_wakeup_signal.
type TeamBus interface {
	// InboxPush appends a message to the session's inbox queue.
	InboxPush(ctx context.Context, sessionID string, msg TeamMessage) error
	// InboxDrain reads and clears all pending inbox messages for the session.
	InboxDrain(ctx context.Context, sessionID string) ([]TeamMessage, error)
	// EnqueueWakeup signals that the session should be re-run to process its inbox.
	EnqueueWakeup(ctx context.Context, sessionID string) error
	// SubscribeWakeup returns a channel of wakeup signals (one consumer is typical).
	SubscribeWakeup(ctx context.Context) (<-chan WakeupEvent, Cancel, error)
}

// AsTeamBus returns a TeamBus view of b if it implements TeamBus, else nil.
// Use this to opt into team-collaboration primitives when a bus supports them.
func AsTeamBus(b Bus) TeamBus {
	if tb, ok := b.(TeamBus); ok {
		return tb
	}
	return nil
}

// --- LocalBus TeamBus implementation ---

func (b *LocalBus) InboxPush(ctx context.Context, sessionID string, msg TeamMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	if msg.SentAt.IsZero() {
		msg.SentAt = time.Now()
	}
	b.inbox[sessionID] = append(b.inbox[sessionID], msg)
	return nil
}

func (b *LocalBus) InboxDrain(ctx context.Context, sessionID string) ([]TeamMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrClosed
	}
	msgs := b.inbox[sessionID]
	delete(b.inbox, sessionID)
	return msgs, nil
}

func (b *LocalBus) EnqueueWakeup(ctx context.Context, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return ErrClosed
	}
	for _, ch := range b.wakeSubs {
		select {
		case ch <- WakeupEvent{SessionID: sessionID}:
		default:
			// Drop on full buffer; the pending inbox is persistent so a missed
			// wakeup just delays processing until the next enqueue.
		}
	}
	return nil
}

func (b *LocalBus) SubscribeWakeup(ctx context.Context) (<-chan WakeupEvent, Cancel, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, ErrClosed
	}
	b.wakeNext++
	id := b.wakeNext
	ch := make(chan WakeupEvent, defaultSubscriberBuffer)
	b.wakeSubs[id] = ch
	var once sync.Once
	cancel := Cancel(func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.wakeSubs, id)
			close(ch)
		})
	})
	return ch, cancel, nil
}
