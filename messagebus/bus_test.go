package messagebus_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/linkerlin/agentscope.go/messagebus"
	bredis "github.com/redis/go-redis/v9"
)

// recv pulls one message from ch or fails the test on timeout.
func recv(t *testing.T, ch <-chan messagebus.Message, timeout time.Duration) messagebus.Message {
	t.Helper()
	select {
	case m, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before a message arrived")
		}
		return m
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message")
	}
	return messagebus.Message{}
}

func TestLocalBus_PubSub(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()

	ch, cancel, err := bus.Subscribe(context.Background(), "events")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	if err := bus.Publish(context.Background(), "events", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	m := recv(t, ch, time.Second)
	if m.Channel != "events" || string(m.Payload) != "hello" {
		t.Fatalf("unexpected message: %+v", m)
	}
}

func TestLocalBus_MultipleSubscribersAndIsolation(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()

	chA, cancelA, _ := bus.Subscribe(context.Background(), "topic")
	chB, cancelB, _ := bus.Subscribe(context.Background(), "topic", "other")
	defer cancelA()
	defer cancelB()

	_ = bus.Publish(context.Background(), "topic", []byte("both"))
	ma := recv(t, chA, time.Second)
	mb := recv(t, chB, time.Second)
	if string(ma.Payload) != "both" || string(mb.Payload) != "both" {
		t.Fatalf("both subscribers should receive: %+v %+v", ma, mb)
	}

	// chA is not subscribed to "other"; only chB receives.
	_ = bus.Publish(context.Background(), "other", []byte("only-b"))
	mb2 := recv(t, chB, time.Second)
	if string(mb2.Payload) != "only-b" {
		t.Fatalf("chB should receive 'other': %+v", mb2)
	}
	select {
	case <-chA:
		t.Fatal("chA must not receive messages from 'other'")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestLocalBus_CancelUnsubscribes(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()

	ch, cancel, _ := bus.Subscribe(context.Background(), "x")
	cancel()
	// After cancel the channel is closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after cancel")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected channel closure after cancel")
	}
	// Publish after cancel must not panic and must not deliver.
	if err := bus.Publish(context.Background(), "x", []byte("late")); err != nil {
		t.Fatalf("publish after cancel: %v", err)
	}
}

func TestLocalBus_PublishDropsOnFullBuffer(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()

	// Subscriber that never drains; buffer is bounded so Publish must not block.
	_, cancel, _ := bus.Subscribe(context.Background(), "flood")
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			_ = bus.Publish(context.Background(), "flood", []byte("x"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish should not block on a full subscriber buffer")
	}
}

func TestLocalBus_CloseErrors(t *testing.T) {
	bus := messagebus.NewLocalBus()
	_ = bus.Close()
	if err := bus.Publish(context.Background(), "x", nil); err != messagebus.ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if _, _, err := bus.Subscribe(context.Background(), "x"); err != messagebus.ErrClosed {
		t.Fatalf("expected ErrClosed on subscribe, got %v", err)
	}
}

func TestRedisBus_PubSub(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	client := bredis.NewClient(&bredis.Options{Addr: mr.Addr()})
	defer client.Close()

	bus := messagebus.NewRedisBus(client, "")
	ch, cancel, err := bus.Subscribe(context.Background(), "ctrl")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	// Allow the subscription goroutine to register before publishing.
	time.Sleep(50 * time.Millisecond)
	if err := bus.Publish(context.Background(), "ctrl", []byte("cancel-task-1")); err != nil {
		t.Fatal(err)
	}
	m := recv(t, ch, 2*time.Second)
	if string(m.Payload) != "cancel-task-1" {
		t.Fatalf("unexpected payload: %q", string(m.Payload))
	}
}

func TestRedisBus_CancelClosesChannel(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	client := bredis.NewClient(&bredis.Options{Addr: mr.Addr()})
	defer client.Close()

	bus := messagebus.NewRedisBus(client, "")
	ch, cancel, _ := bus.Subscribe(context.Background(), "x")
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("expected channel closure after cancel")
	}
}

func TestLocalBus_PublishWithNoSubscribers(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	// Publishing to a channel with no subscribers is a no-op (no error/panic).
	if err := bus.Publish(context.Background(), "ghost", []byte("x")); err != nil {
		t.Fatalf("publish no subscribers: %v", err)
	}
}

func TestLocalBus_ContextCancelledPublish(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := bus.Publish(ctx, "x", []byte("y")); err == nil {
		t.Fatal("expected error when publish context cancelled")
	}
}

func TestRedisBus_NilClientErrors(t *testing.T) {
	bus := messagebus.NewRedisBus(nil, "")
	if err := bus.Publish(context.Background(), "x", []byte("y")); err != messagebus.ErrClosed {
		t.Fatalf("expected ErrClosed on nil client publish, got %v", err)
	}
	if _, _, err := bus.Subscribe(context.Background(), "x"); err != messagebus.ErrClosed {
		t.Fatalf("expected ErrClosed on nil client subscribe, got %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("Close should be no-op: %v", err)
	}
}

func TestRedisBus_SubscribeNoChannels(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	client := bredis.NewClient(&bredis.Options{Addr: mr.Addr()})
	defer client.Close()

	bus := messagebus.NewRedisBus(client, "")
	ch, cancel, err := bus.Subscribe(context.Background())
	if ch != nil || cancel != nil || err != nil {
		t.Fatalf("expected nil returns for no channels, got ch=%v cancel=%v err=%v", ch, cancel, err)
	}
}
