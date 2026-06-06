package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestInitTracerProvider(t *testing.T) {
	tp, err := InitTracerProvider("test-service")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	defer tp.Shutdown(context.Background())

	tracer := otel.Tracer("test")
	if tracer == nil {
		t.Fatal("expected tracer")
	}
}

func TestOtelTracer(t *testing.T) {
	tp, err := InitTracerProvider("test-service")
	if err != nil {
		t.Fatal(err)
	}
	defer tp.Shutdown(context.Background())

	tracer := NewOtelTracer(otel.Tracer("test"))
	ctx, span := tracer.Start(context.Background(), "test-span")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	span.End()
}
