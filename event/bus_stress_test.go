package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBus_StressManySubscribers(t *testing.T) {
	bus := NewBus(100)
	const subs = 50
	const events = 100

	var received []int64
	var mu sync.Mutex
	var ids []string

	for i := 0; i < subs; i++ {
		id, ch := bus.Subscribe()
		ids = append(ids, id)
		go func() {
			var count int64
			for range ch {
				count++
			}
			mu.Lock()
			received = append(received, count)
			mu.Unlock()
		}()
	}

	for i := 0; i < events; i++ {
		bus.Publish(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "x"})
	}

	// Give consumers time to process
	time.Sleep(200 * time.Millisecond)

	// Unsubscribe to let consumers exit
	for _, id := range ids {
		bus.Unsubscribe(id)
	}
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	total := int64(0)
	for _, c := range received {
		total += c
	}
	mu.Unlock()

	// Every subscriber should have received all events (buffer is large enough)
	if total != int64(subs*events) {
		t.Fatalf("expected %d total receptions, got %d", subs*events, total)
	}
}

func TestBus_StressRapidPublish(t *testing.T) {
	bus := NewBus(10)
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	const n = 10000
	var received int64

	done := make(chan bool)
	go func() {
		for range ch {
			atomic.AddInt64(&received, 1)
		}
		done <- true
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < n/10; j++ {
				bus.Publish(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "x"})
			}
		}()
	}
	wg.Wait()

	// Allow time for last events to be delivered
	time.Sleep(100 * time.Millisecond)
	bus.Unsubscribe(id)
	<-done

	// With a small buffer, some events may be dropped; we just verify it didn't deadlock
	if atomic.LoadInt64(&received) == 0 {
		t.Fatal("expected at least some events to be received")
	}
}

func TestBus_StressSubscribeUnsubscribe(t *testing.T) {
	bus := NewBus(10)
	const iterations = 1000

	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, ch := bus.Subscribe()
			bus.Publish(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: TypeTextBlockDelta, Ts: time.Now(), ReplyID_: "r1"}, Delta: "x"})
			select {
			case <-ch:
			case <-time.After(50 * time.Millisecond):
			}
			bus.Unsubscribe(id)
		}()
	}
	wg.Wait()

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", bus.SubscriberCount())
	}
}
