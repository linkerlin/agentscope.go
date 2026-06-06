package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestOTelMiddleware(t *testing.T) {
	// Setup in-memory tracer and meter for testing.
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(nil)
	mp := sdkmetric.NewMeterProvider()
	defer mp.Shutdown(nil)

	tracer := tp.Tracer("test")
	meter := mp.Meter("test")

	otel, err := NewOTelMiddleware(tracer, meter)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	handler := otel.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestStatusRecorder(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, statusCode: http.StatusOK}
	sr.WriteHeader(http.StatusTeapot)
	if sr.statusCode != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", sr.statusCode)
	}
}

func TestWithOTelTracing(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(nil)
	mp := sdkmetric.NewMeterProvider()
	defer mp.Shutdown(nil)

	srv := NewServer(&mockAgent{name: "test"})
	if err := srv.WithOTelTracing(tp.Tracer("test"), mp.Meter("test")); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// Verify route is still accessible.
	body := `{"text":"hello"}`
	req := httptest.NewRequest("POST", "/chat", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Ensure we don't break the global OTel state.
func TestOTelGlobalState(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(nil)

	tracer := otel.Tracer("agentscope")
	_, span := tracer.Start(nil, "test") // nil context should be handled gracefully
	span.End()
}
