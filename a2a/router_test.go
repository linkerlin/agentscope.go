package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShardRouter_Route(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(AgentCard{Name: "a", URL: "http://a"})
	_ = reg.Register(AgentCard{Name: "b", URL: "http://b"})
	_ = reg.Register(AgentCard{Name: "c", URL: "http://c"})

	router := NewShardRouter(reg, 100)
	if err := router.Refresh(); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"user-1", "user-2", "user-3", "session-x", "task-42"} {
		url, err := router.Route(key)
		if err != nil {
			t.Fatalf("route %s: %v", key, err)
		}
		if url == "" {
			t.Fatalf("expected non-empty url for %s", key)
		}
	}
}

func TestShardRouter_SkipsUnhealthy(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(AgentCard{Name: "healthy", Version: "1.0.0"})
	}))
	defer healthyServer.Close()

	reg := NewRegistry()
	_ = reg.Register(AgentCard{Name: "healthy", URL: healthyServer.URL})
	_ = reg.Register(AgentCard{Name: "unhealthy", URL: "http://unhealthy"})

	reg.HealthCheck(context.Background())

	router := NewShardRouter(reg, 50)
	if err := router.Refresh(); err != nil {
		t.Fatal(err)
	}

	// The unhealthy node should not be on the ring because health check failed.
	if router.HasNode("http://unhealthy") {
		t.Fatal("expected unhealthy node to be excluded")
	}
	if !router.HasNode(healthyServer.URL) {
		t.Fatal("expected healthy node to be present")
	}
}

func TestShardRouter_EmptyRing(t *testing.T) {
	reg := NewRegistry()
	router := NewShardRouter(reg, 10)
	_ = router.Refresh()
	if _, err := router.Route("x"); err == nil {
		t.Fatal("expected error for empty ring")
	}
}

func TestShardRouter_Deterministic(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(AgentCard{Name: "a", URL: "http://a"})
	_ = reg.Register(AgentCard{Name: "b", URL: "http://b"})

	router := NewShardRouter(reg, 100)
	_ = router.Refresh()

	url1, err := router.Route("same-key")
	if err != nil {
		t.Fatal(err)
	}
	url2, err := router.Route("same-key")
	if err != nil {
		t.Fatal(err)
	}
	if url1 != url2 {
		t.Fatalf("expected deterministic routing, got %s and %s", url1, url2)
	}
}
