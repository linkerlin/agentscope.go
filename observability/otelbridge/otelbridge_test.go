package otelbridge

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/observability"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceContextFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	tc := observability.TraceContextFromContext(ctx)
	if tc.IsValid() {
		t.Fatal("expected invalid trace context without span")
	}
}

func TestTraceContextFromContext_WithSpan(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	tc := observability.TraceContextFromContext(ctx)
	if !tc.IsValid() {
		t.Fatalf("expected valid trace context, got trace_id=%s span_id=%s", tc.TraceID, tc.SpanID)
	}
	if tc.TraceID != traceID.String() {
		t.Fatalf("expected trace_id %s, got %s", traceID.String(), tc.TraceID)
	}
	if tc.SpanID != spanID.String() {
		t.Fatalf("expected span_id %s, got %s", spanID.String(), tc.SpanID)
	}
}
