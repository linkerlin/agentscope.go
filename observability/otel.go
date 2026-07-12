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
	// SetAttributes attaches semantic key/value attributes to the span. Mirrors
	// OTel span.SetAttributes. No-op for tracers that discard attributes.
	SetAttributes(attrs ...SpanAttr)
}

// SpanAttr is a single key/value span attribute. Value may be string, int,
// int64, float64, or bool. Mirrors OTel attribute semantics.
type SpanAttr struct {
	Key   string
	Value any
}

// SpanStatus codes, mirroring OTel codes.
type SpanStatus int

const (
	StatusUnset SpanStatus = iota
	StatusOK
	StatusError
)

// Attr helpers build SpanAttr values concisely.
func StringAttr(k, v string) SpanAttr          { return SpanAttr{Key: k, Value: v} }
func IntAttr(k string, v int) SpanAttr         { return SpanAttr{Key: k, Value: v} }
func Int64Attr(k string, v int64) SpanAttr     { return SpanAttr{Key: k, Value: v} }
func Float64Attr(k string, v float64) SpanAttr { return SpanAttr{Key: k, Value: v} }
func BoolAttr(k string, v bool) SpanAttr       { return SpanAttr{Key: k, Value: v} }

// noopSpan is used when no tracer is configured.
type noopSpan struct{}

func (noopSpan) End()                      {}
func (noopSpan) RecordError(error)         {}
func (noopSpan) SetAttributes(...SpanAttr) {}

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
