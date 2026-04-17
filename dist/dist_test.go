package dist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/a2a"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	card := a2a.AgentCard{Name: "A", URL: "http://a.com"}
	r.Register(card)

	if len(r.List()) != 1 {
		t.Fatalf("expected 1 card, got %d", len(r.List()))
	}
	c, ok := r.Get("http://a.com")
	if !ok || c.Name != "A" {
		t.Fatal("expected to get card A")
	}

	r.Deregister("http://a.com")
	if len(r.List()) != 0 {
		t.Fatalf("expected 0 cards after deregister")
	}
}

func TestDiscover(t *testing.T) {
	card := a2a.AgentCard{Name: "Remote", URL: "http://example.com"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			_ = json.NewEncoder(w).Encode(card)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	got, err := Discover(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != card.Name {
		t.Fatalf("expected name %s, got %s", card.Name, got.Name)
	}
}

func TestAutoDiscover(t *testing.T) {
	card := a2a.AgentCard{Name: "Auto", URL: "http://auto.com"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			_ = json.NewEncoder(w).Encode(card)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	r := NewRegistry()
	ad := NewAutoDiscover(r, []string{srv.URL}, 50*time.Millisecond, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ad.Start(ctx)
	<-ctx.Done()
	ad.Stop()

	if len(r.List()) == 0 {
		t.Fatal("expected at least one discovery")
	}
}

func TestCoordinator_Random(t *testing.T) {
	var called bool
	coord := NewCoordinator(NewRegistry(), Random)
	coord.SetClientFactory(func(baseURL string) a2a.Client {
		return &fakeClient{sendFn: func(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
			called = true
			return &a2a.Message{Role: "agent", Content: "pong"}, nil
		}}
	})
	coord.registry.Register(a2a.AgentCard{Name: "A", URL: "http://a"})

	resp, err := coord.Send(context.Background(), &a2a.Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected client to be called")
	}
	if resp.Content != "pong" {
		t.Fatalf("unexpected response: %s", resp.Content)
	}
}

func TestCoordinator_RoundRobin(t *testing.T) {
	var order []string
	coord := NewCoordinator(NewRegistry(), RoundRobin)
	coord.SetClientFactory(func(baseURL string) a2a.Client {
		return &fakeClient{sendFn: func(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
			order = append(order, baseURL)
			return &a2a.Message{Role: "agent", Content: baseURL}, nil
		}}
	})
	coord.registry.Register(a2a.AgentCard{Name: "A", URL: "http://a"})
	coord.registry.Register(a2a.AgentCard{Name: "B", URL: "http://b"})

	_, _ = coord.Send(context.Background(), &a2a.Message{Role: "user", Content: "hi"})
	_, _ = coord.Send(context.Background(), &a2a.Message{Role: "user", Content: "hi"})

	if len(order) != 2 || order[0] != "http://a" || order[1] != "http://b" {
		t.Fatalf("expected round-robin [http://a http://b], got %v", order)
	}
}

func TestCoordinator_Broadcast(t *testing.T) {
	coord := NewCoordinator(NewRegistry(), Broadcast)
	coord.SetClientFactory(func(baseURL string) a2a.Client {
		return &fakeClient{sendFn: func(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
			return &a2a.Message{Role: "agent", Content: baseURL}, nil
		}}
	})
	coord.registry.Register(a2a.AgentCard{Name: "A", URL: "http://a"})
	coord.registry.Register(a2a.AgentCard{Name: "B", URL: "http://b"})

	resp, err := coord.Send(context.Background(), &a2a.Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Content == "" {
		t.Fatal("expected non-empty broadcast response")
	}
}

func TestCoordinator_Broadcast_AllFail(t *testing.T) {
	coord := NewCoordinator(NewRegistry(), Broadcast)
	coord.SetClientFactory(func(baseURL string) a2a.Client {
		return &fakeClient{sendFn: func(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
			return nil, errors.New("fail")
		}}
	})
	coord.registry.Register(a2a.AgentCard{Name: "A", URL: "http://a"})

	_, err := coord.Send(context.Background(), &a2a.Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCoordinator_EmptyRegistry(t *testing.T) {
	coord := NewCoordinator(NewRegistry(), Random)
	_, err := coord.Send(context.Background(), &a2a.Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error for empty registry")
	}
}

func TestCoordinator_SendTo(t *testing.T) {
	coord := NewCoordinator(NewRegistry(), Random)
	coord.SetClientFactory(func(baseURL string) a2a.Client {
		return &fakeClient{sendFn: func(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
			return &a2a.Message{Role: "agent", Content: "pong"}, nil
		}}
	})
	coord.registry.Register(a2a.AgentCard{Name: "A", URL: "http://a"})

	resp, err := coord.SendTo(context.Background(), "http://a", &a2a.Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "pong" {
		t.Fatalf("unexpected response: %s", resp.Content)
	}
}

type fakeClient struct {
	sendFn func(ctx context.Context, msg *a2a.Message) (*a2a.Message, error)
}

func (f *fakeClient) Send(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
	return f.sendFn(ctx, msg)
}

func (f *fakeClient) SendSubscribe(ctx context.Context, msg *a2a.Message) (<-chan *a2a.Message, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) Close() error { return nil }
