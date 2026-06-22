package messagebus_test

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/messagebus"
)

func TestLocalBus_Inbox(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()
	if err := bus.InboxPush(ctx, "s1", messagebus.TeamMessage{From: "a", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	_ = bus.InboxPush(ctx, "s1", messagebus.TeamMessage{From: "b", Content: "yo"})
	msgs, err := bus.InboxDrain(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	// Drain clears the inbox.
	if msgs2, _ := bus.InboxDrain(ctx, "s1"); len(msgs2) != 0 {
		t.Fatalf("expected empty after drain, got %d", len(msgs2))
	}
}

func TestLocalBus_Wakeup(t *testing.T) {
	bus := messagebus.NewLocalBus()
	defer bus.Close()
	ctx := context.Background()
	ch, cancel, err := bus.SubscribeWakeup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if err := bus.EnqueueWakeup(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.SessionID != "s1" {
			t.Fatalf("expected s1, got %s", ev.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for wakeup")
	}
}

func TestAsTeamBus(t *testing.T) {
	if messagebus.AsTeamBus(messagebus.NewLocalBus()) == nil {
		t.Fatal("LocalBus should implement TeamBus")
	}
	if messagebus.AsTeamBus(nil) != nil {
		t.Fatal("nil bus should yield nil TeamBus")
	}
}
