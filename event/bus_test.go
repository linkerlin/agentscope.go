package event

import (
	"context"
	"testing"
	"time"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus(10)
	id, ch, _ := bus.Subscribe()
	defer bus.Unsubscribe(id)

	ev := &TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "hello"}
	bus.Publish(ev)

	select {
	case got := <-ch:
		if got.EventType() != TypeTextBlockDelta {
			t.Fatalf("unexpected event type: %s", got.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus(10)
	id1, ch1, _ := bus.Subscribe()
	id2, ch2, _ := bus.Subscribe()
	defer bus.Unsubscribe(id1)
	defer bus.Unsubscribe(id2)

	ev := &TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "hi"}
	bus.Publish(ev)

	var got1, got2 AgentEvent
	select {
	case got1 = <-ch1:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ch1")
	}
	select {
	case got2 = <-ch2:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ch2")
	}
	if got1 == nil || got2 == nil {
		t.Fatal("expected both subscribers to receive event")
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus(10)
	id, ch, done := bus.Subscribe()
	bus.Unsubscribe(id)

	// ch is intentionally not closed by Unsubscribe (to avoid send/close races);
	// done is closed instead. Receiver should select on done or use ctx.
	select {
	case <-done:
		// expected
	default:
		t.Fatal("expected done to be closed after unsubscribe")
	}
	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", bus.SubscriberCount())
	}
	// ch may still deliver a buffered event; we do not assert close here.
	_ = ch
}

func TestBus_Forward(t *testing.T) {
	bus := NewBus(10)
	id, ch, _ := bus.Subscribe()
	defer bus.Unsubscribe(id)

	src := make(chan AgentEvent, 2)
	src <- &TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "a"}
	src <- &TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "b"}
	close(src)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bus.Forward(ctx, src)

	var deltas []string
	for ev := range ch {
		if e, ok := ev.(*TextBlockDeltaEvent); ok {
			deltas = append(deltas, e.Delta)
		}
		if len(deltas) >= 2 {
			break
		}
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d", len(deltas))
	}
}

func TestBus_DropSlowConsumer(t *testing.T) {
	bus := NewBus(1) // tiny buffer
	id, ch, _ := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Fill the buffer
	bus.Publish(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "1"})
	bus.Publish(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "2"})
	bus.Publish(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "3"})

	// Should not block; slow consumer simply drops.
	// Drain what we can.
	count := 0
drain:
	for {
		select {
		case <-ch:
			count++
		case <-time.After(100 * time.Millisecond):
			break drain
		}
	}
	if count < 1 {
		t.Fatal("expected at least one event to be buffered")
	}
}
