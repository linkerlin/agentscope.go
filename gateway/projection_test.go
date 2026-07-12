package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkerlin/agentscope.go/messagebus"
)

func TestSessionProjection_LocalBus(t *testing.T) {
	bus := messagebus.NewLocalBus()
	sp := NewSessionProjection(bus)
	ctx := context.Background()

	if err := sp.Project(ctx, "leader-1", "hitl-worker-a", []byte(`{"prompt":"approve?"}`)); err != nil {
		t.Fatal(err)
	}
	sp.Project(ctx, "leader-1", "hitl-worker-b", []byte(`{"prompt":"deploy?"}`))

	cards, err := sp.List(ctx, "leader-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 projections, got %d", len(cards))
	}
	if string(cards["hitl-worker-a"]) != `{"prompt":"approve?"}` {
		t.Fatalf("unexpected card: %q", cards["hitl-worker-a"])
	}

	// Clear one
	sp.Clear(ctx, "leader-1", "hitl-worker-a")
	cards, _ = sp.List(ctx, "leader-1")
	if len(cards) != 1 {
		t.Fatalf("expected 1 after clear, got %d", len(cards))
	}

	// Isolation: another session sees nothing.
	cards, _ = sp.List(ctx, "leader-2")
	if len(cards) != 0 {
		t.Fatalf("session isolation broken: %d", len(cards))
	}
}

func TestSessionProjection_NoopWithoutCoordBus(t *testing.T) {
	// A nil coord (e.g. a bus that is not a CoordBus) yields empty results and
	// no errors — graceful degradation.
	sp := &SessionProjection{coord: nil}
	cards, err := sp.List(context.Background(), "s1")
	if err != nil || len(cards) != 0 {
		t.Fatalf("nil coord should no-op: %v %d", err, len(cards))
	}
	if err := sp.Project(context.Background(), "s1", "k", []byte("v")); err != nil {
		t.Fatal("nil coord Project should not error")
	}
}

func TestSessionProjection_HTTPRoutes(t *testing.T) {
	bus := messagebus.NewLocalBus()
	srv := NewServer(&mockAgent{}).WithMessageBus(bus)
	srv.RegisterProjectionRoutes()

	sp := NewSessionProjection(bus)
	sp.Project(context.Background(), "sess-1", "card-1", []byte(`{"x":1}`))

	// GET projections
	req := httptest.NewRequest("GET", "/api/v1/sessions/sess-1/projections", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d %s", w.Code, w.Body.String())
	}

	// DELETE projection
	req = httptest.NewRequest("DELETE", "/api/v1/sessions/sess-1/projections/card-1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d", w.Code)
	}

	// Verify cleared
	cards, _ := sp.List(context.Background(), "sess-1")
	if len(cards) != 0 {
		t.Fatalf("expected clear, got %d", len(cards))
	}
}

func TestSessionProjection_HTTPNoBus(t *testing.T) {
	// Server without a bus: routes should not register, so 404.
	srv := NewServer(&mockAgent{})
	srv.RegisterProjectionRoutes()
	req := httptest.NewRequest("GET", "/api/v1/sessions/sess-1/projections", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without bus, got %d", w.Code)
	}
}
