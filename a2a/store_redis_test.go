package a2a

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *RedisRegistryStore) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	store := NewRedisRegistryStore(client, "test:a2a")
	return srv, store
}

func TestRedisRegistryStore_RegisterAndGet(t *testing.T) {
	_, store := setupMiniRedis(t)
	ctx := context.Background()
	entry := RegistryEntry{
		Card:         AgentCard{Name: "agent-1", URL: "http://agent-1:8080"},
		DiscoveredAt: time.Now(),
		LastSeen:     time.Now(),
		Healthy:      true,
	}
	if err := store.Register(ctx, entry); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, entry.Card.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected entry")
	}
	if got.Card.Name != "agent-1" {
		t.Fatalf("name mismatch: %s", got.Card.Name)
	}
}

func TestRedisRegistryStore_RegisterMissingURL(t *testing.T) {
	_, store := setupMiniRedis(t)
	ctx := context.Background()
	entry := RegistryEntry{Card: AgentCard{Name: "bad"}}
	if err := store.Register(ctx, entry); err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestRedisRegistryStore_ListAndRemove(t *testing.T) {
	_, store := setupMiniRedis(t)
	ctx := context.Background()
	entries := []RegistryEntry{
		{Card: AgentCard{Name: "a", URL: "http://a"}, Healthy: true},
		{Card: AgentCard{Name: "b", URL: "http://b"}, Healthy: true},
	}
	for _, e := range entries {
		if err := store.Register(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}

	if err := store.Remove(ctx, "http://a"); err != nil {
		t.Fatal(err)
	}
	list, err = store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 entry after remove, got %d", len(list))
	}
}

func TestRedisRegistryStore_UpdateHealth(t *testing.T) {
	_, store := setupMiniRedis(t)
	ctx := context.Background()
	entry := RegistryEntry{
		Card:    AgentCard{Name: "hc", URL: "http://hc"},
		Healthy: true,
	}
	if err := store.Register(ctx, entry); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := store.UpdateHealth(ctx, entry.Card.URL, false, now); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, entry.Card.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got.Healthy {
		t.Fatal("expected unhealthy")
	}
	if !got.LastSeen.Equal(now) {
		t.Fatalf("last seen mismatch: got %v want %v", got.LastSeen, now)
	}
}

func TestRedisRegistryStore_GetMissing(t *testing.T) {
	_, store := setupMiniRedis(t)
	ctx := context.Background()
	got, err := store.Get(ctx, "http://missing")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil for missing entry")
	}
}
