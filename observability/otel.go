package observability

import (
	"context"
)

// TraceContext holds OpenTelemetry trace and span identifiers.
type TraceContext struct {
	TraceID string
	SpanID  string
}

// IsValid reports whether both identifiers are non-empty.
func (tc TraceContext) IsValid() bool {
	return tc.TraceID != "" && tc.SpanID != ""
}

// TraceContextExtractor extracts trace context from a context.Context.
// The default implementation returns an empty TraceContext.
// Users who import the optional package `observability/otelbridge`
// will automatically get a real OTel-backed extractor.
type TraceContextExtractor func(context.Context) TraceContext

// TraceContextFromContext is used by JsonlTraceExporter and TracedAgent
// to optionally attach trace_id/span_id to records.
// It is safe to call even when OpenTelemetry is not on the classpath.
var TraceContextFromContext TraceContextExtractor = func(context.Context) TraceContext {
	return TraceContext{}
}

// Span represents a minimal abstraction for an OpenTelemetry-like span.
type Span interface {
	End()
	RecordError(err error)
}

// noopSpan is used when no tracer is configured.
type noopSpan struct{}

func (noopSpan) End()             {}
func (noopSpan) RecordError(error) {}

// Tracer is a minimal tracer abstraction used by TracedAgent.
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
}

// NoopTracer is a tracer that does nothing.
var NoopTracer Tracer = noopTracer{}

type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}
