package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	card := AgentCard{
		Name:         "test-agent",
		URL:          "http://localhost:8080",
		Version:      "1.0.0",
		Capabilities: []string{"streaming"},
	}
	if err := r.Register(card); err != nil {
		t.Fatal(err)
	}

	got, ok := r.Get("http://localhost:8080")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if got.Card.Name != "test-agent" {
		t.Fatalf("name mismatch: %s", got.Card.Name)
	}
}

func TestRegistry_RegisterMissingURL(t *testing.T) {
	r := NewRegistry()
	card := AgentCard{Name: "bad"}
	if err := r.Register(card); err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("http://nope"); ok {
		t.Fatal("expected not found for unknown agent")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(AgentCard{Name: "a1", URL: "http://a1"})
	r.Register(AgentCard{Name: "a2", URL: "http://a2"})

	entries := r.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	r.Register(AgentCard{Name: "a1", URL: "http://a1"})
	r.Remove("http://a1")
	if _, ok := r.Get("http://a1"); ok {
		t.Fatal("expected not found after remove")
	}
}

func TestRegistry_Discover(t *testing.T) {
	card := AgentCard{Name: "remote", URL: "", Version: "1.0.0"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/agent.json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	r := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Discover(ctx, server.URL); err != nil {
		t.Fatal(err)
	}

	got, ok := r.Get(server.URL)
	if !ok {
		t.Fatal("expected discovered agent to be registered")
	}
	if got.Card.Name != "remote" {
		t.Fatalf("name mismatch: %s", got.Card.Name)
	}
	if got.Card.URL != server.URL {
		t.Fatalf("URL not set: %s", got.Card.URL)
	}
}

func TestRegistry_DiscoverNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	r := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Discover(ctx, server.URL); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestRegistry_HealthCheck(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(AgentCard{Name: "hc", Version: "1.0.0"})
	}))
	defer server.Close()

	r := NewRegistry()
	r.Register(AgentCard{Name: "hc", URL: server.URL})

	entry, _ := r.Get(server.URL)
	if !entry.Healthy {
		t.Fatal("expected healthy initially")
	}

	// After health check, server responds OK on first call.
	r.HealthCheck(context.Background())
	entry, _ = r.Get(server.URL)
	if !entry.Healthy {
		t.Fatal("expected healthy after first check")
	}

	// Second health check should mark unhealthy.
	r.HealthCheck(context.Background())
	entry, _ = r.Get(server.URL)
	if entry.Healthy {
		t.Fatal("expected unhealthy after second check")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			_ = r.Register(AgentCard{Name: "a", URL: "http://a" + string(rune('0'+idx))})
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		go func() {
			_ = r.List()
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
