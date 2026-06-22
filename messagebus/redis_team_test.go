package messagebus_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"
	bredis "github.com/redis/go-redis/v9"
)

// redisTestBus builds a RedisBus against $REDIS_URL with a unique prefix so
// parallel test runs don't collide. Skips when REDIS_URL is unset or the server
// is unreachable. Best-effort cleanup of the wakeup stream is registered.
func redisTestBus(t *testing.T) *messagebus.RedisBus {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set; skipping Redis integration test")
	}
	opts, err := bredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL %q: %v", url, err)
	}
	client := bredis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		t.Skipf("redis unreachable at %s: %v", url, err)
	}
	prefix := "as:test:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	streamKey := prefix + ":wakeup:stream"
	t.Cleanup(func() {
		ctx := context.Background()
		_ = client.Del(ctx, streamKey).Err()
		_ = client.Close()
	})
	return messagebus.NewRedisBus(client, prefix)
}

// TestRedisBus_WakeupFanOut verifies multiple consumers all receive the same
// wakeup (true fan-out via Redis Streams), which is the multi-process guarantee
// the single-consumer BLPOP could not provide.
func TestRedisBus_WakeupFanOut(t *testing.T) {
	bus := redisTestBus(t)
	ctx := context.Background()

	ch1, c1, err := bus.SubscribeWakeup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c1()
	ch2, c2, err := bus.SubscribeWakeup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c2()

	// Give both XREAD loops a moment to start.
	time.Sleep(300 * time.Millisecond)
	if err := bus.EnqueueWakeup(ctx, "session-fanout"); err != nil {
		t.Fatal(err)
	}

	for i, ch := range []<-chan messagebus.WakeupEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.SessionID != "session-fanout" {
				t.Errorf("consumer %d: got %q, want session-fanout", i, ev.SessionID)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("consumer %d timed out waiting for wakeup (fan-out failed)", i)
		}
	}
}

// TestRedisBus_WakeupBacklog verifies wakeups enqueued BEFORE a subscriber
// connected are not lost (durable stream, read from "0").
func TestRedisBus_WakeupBacklog(t *testing.T) {
	bus := redisTestBus(t)
	ctx := context.Background()

	if err := bus.EnqueueWakeup(ctx, "session-backlog"); err != nil {
		t.Fatal(err)
	}
	ch, cancel, err := bus.SubscribeWakeup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	select {
	case ev := <-ch:
		if ev.SessionID != "session-backlog" {
			t.Errorf("got %q, want session-backlog", ev.SessionID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backlog wakeup not received (durability failed)")
	}
}

// TestRedisBus_Inbox verifies cross-process inbox push/drain with atomic clear.
func TestRedisBus_Inbox(t *testing.T) {
	bus := redisTestBus(t)
	ctx := context.Background()

	if err := bus.InboxPush(ctx, "s1", messagebus.TeamMessage{From: "a", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := bus.InboxPush(ctx, "s1", messagebus.TeamMessage{From: "b", Content: "yo"}); err != nil {
		t.Fatal(err)
	}
	msgs, err := bus.InboxDrain(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs2, _ := bus.InboxDrain(ctx, "s1"); len(msgs2) != 0 {
		t.Fatalf("inbox should be empty after drain, got %d", len(msgs2))
	}
}
