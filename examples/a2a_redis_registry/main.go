// a2a_redis_registry demonstrates a distributed A2A agent registry backed by
// Redis and consistent-hash request routing across healthy agents.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/linkerlin/agentscope.go/a2a"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// Use a real Redis if REDIS_URL is set; otherwise spin up an embedded
	// miniredis so the example is runnable without external dependencies.
	var client redis.UniversalClient
	var cleanup func()
	if addr := os.Getenv("REDIS_URL"); addr != "" {
		client = redis.NewClient(&redis.Options{Addr: addr})
		cleanup = func() {}
		fmt.Println("Using Redis at", addr)
	} else {
		srv, err := miniredis.Run()
		if err != nil {
			log.Fatal(err)
		}
		client = redis.NewClient(&redis.Options{Addr: srv.Addr()})
		cleanup = srv.Close
		fmt.Println("Using embedded miniredis at", srv.Addr())
	}
	defer cleanup()

	store := a2a.NewRedisRegistryStore(client, "example:a2a")
	registry := a2a.NewRegistryWithStore(store)

	// Spin up two fake agent servers and register them.
	servers := []*httptest.Server{
		startFakeAgent("agent-alpha"),
		startFakeAgent("agent-beta"),
	}
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	for _, s := range servers {
		if err := registry.Discover(ctx, s.URL); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("\nRegistered agents:")
	for _, e := range registry.List() {
		fmt.Printf("- %s @ %s (healthy=%v)\n", e.Card.Name, e.Card.URL, e.Healthy)
	}

	// Build a consistent-hash router over healthy agents.
	router := a2a.NewShardRouter(registry, 120)
	if err := router.Refresh(); err != nil {
		log.Fatal(err)
	}

	// Start automatic ring refresh. Local registry changes trigger immediate
	// refresh; polling catches external Redis changes.
	routerCtx, stopRouter := context.WithCancel(ctx)
	defer stopRouter()
	router.AutoRefresh(routerCtx, 2*time.Second)

	fmt.Println("\nRouting samples:")
	for _, key := range []string{"user-42", "session-abc", "task-123", "org-acme", "user-7"} {
		url, err := router.Route(key)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s -> %s\n", key, url)
	}

	// Subscribe to registry changes for observability/logging.
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	watchCh := registry.Watch(watchCtx)
	go func() {
		for change := range watchCh {
			fmt.Printf("[watch] %s %s healthy=%v\n", change.Op, change.URL, change.Healthy)
		}
	}()

	// Simulate a failure of agent-alpha. AutoRefresh will rebuild the ring.
	fmt.Println("\nStopping agent-alpha...")
	servers[0].Close()
	registry.HealthCheck(ctx)
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nAfter agent-alpha failure:")
	for _, key := range []string{"user-42", "session-abc", "task-123", "org-acme", "user-7"} {
		url, err := router.Route(key)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s -> %s\n", key, url)
	}
}

func startFakeAgent(name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/agent.json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(a2a.AgentCard{
			Name:         name,
			Version:      "1.0.0",
			Capabilities: []string{"streaming"},
		})
	}))
}
