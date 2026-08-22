package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/linkerlin/agentscope.go/controlplane"
)

func TestWithMetricsRegistry_ExposesControlPlaneCounters(t *testing.T) {
	k := controlplane.NewKernel(nil, nil, nil)
	_ = k.OpenGate(context.Background(), controlplane.UserGate{GateID: "g", GoalID: "goal", Question: "q"})

	srv := NewServer(&mockAgent{name: "t"}).WithControlPlane(k)
	srv.WithMetricsRegistry(prometheus.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "agentscope_controlplane_gates_opened_total 1") {
		t.Fatalf("control-plane counters missing from /metrics:\n%s", body[:min(400, len(body))])
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("content type: %s", rec.Header().Get("Content-Type"))
	}
}

func TestWithMetricsRegistry_NilRegistryUsesFreshOne(t *testing.T) {
	srv := NewServer(&mockAgent{name: "t"})
	srv.WithMetricsRegistry(nil) // must not panic

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics: expected 200, got %d", rec.Code)
	}
}
