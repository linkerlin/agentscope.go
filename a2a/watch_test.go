package a2a

import (
	"context"
	"testing"
	"time"
)

func TestRegistry_Watch(t *testing.T) {
	reg := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := reg.Watch(ctx)

	if err := reg.Register(AgentCard{Name: "a", URL: "http://a"}); err != nil {
		t.Fatal(err)
	}

	select {
	case change := <-ch:
		if change.Op != ChangeOpRegister {
			t.Fatalf("expected register op, got %s", change.Op)
		}
		if change.URL != "http://a" {
			t.Fatalf("expected http://a, got %s", change.URL)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for register event")
	}

	reg.Remove("http://a")
	select {
	case change := <-ch:
		if change.Op != ChangeOpRemove {
			t.Fatalf("expected remove op, got %s", change.Op)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for remove event")
	}
}

func TestRegistry_WatchHealthChange(t *testing.T) {
	reg := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := reg.Watch(ctx)

	_ = reg.Register(AgentCard{Name: "a", URL: "http://a"})
	<-ch // consume register

	reg.HealthCheck(ctx)
	// No health change because initial state is healthy and no server responds -> unhealthy.
	select {
	case change := <-ch:
		if change.Op != ChangeOpHealth {
			t.Fatalf("expected health op, got %s", change.Op)
		}
		if change.Healthy {
			t.Fatal("expected unhealthy event")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for health event")
	}
}

func TestShardRouter_AutoRefresh(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(AgentCard{Name: "a", URL: "http://a"})
	_ = reg.Register(AgentCard{Name: "b", URL: "http://b"})

	router := NewShardRouter(reg, 50)
	_ = router.Refresh()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.AutoRefresh(ctx, 50*time.Millisecond)

	// Removing a node should trigger a refresh via the watch channel.
	reg.Remove("http://a")
	time.Sleep(100 * time.Millisecond)

	if router.HasNode("http://a") {
		t.Fatal("expected router to refresh and exclude removed node")
	}
	if !router.HasNode("http://b") {
		t.Fatal("expected node b to remain")
	}
}

func TestShardRouter_AutoRefreshPolling(t *testing.T) {
	_, store := setupMiniRedis(t)
	ctx := context.Background()

	reg := NewRegistryWithStore(store)
	_ = reg.Register(AgentCard{Name: "a", URL: "http://a"})
	_ = reg.Register(AgentCard{Name: "b", URL: "http://b"})

	router := NewShardRouter(reg, 50)
	_ = router.Refresh()

	ctxAuto, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.AutoRefresh(ctxAuto, 50*time.Millisecond)

	// Simulate an external store change: mark a unhealthy directly via store.
	if err := store.UpdateHealth(ctx, "http://a", false, time.Now()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(150 * time.Millisecond)
	if router.HasNode("http://a") {
		t.Fatal("expected polling refresh to exclude unhealthy node")
	}
	if !router.HasNode("http://b") {
		t.Fatal("expected node b to remain")
	}
}
