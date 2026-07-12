package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracerProvider creates a TracerProvider with a stdout exporter.
// In production, replace stdouttrace with otlptracehttp or otlptracegrpc.
func InitTracerProvider(serviceName string) (*sdktrace.TracerProvider, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("observability: create trace exporter: %w", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

// ShutdownTracerProvider flushes and shuts down the tracer provider.
func ShutdownTracerProvider(ctx context.Context, tp *sdktrace.TracerProvider) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return tp.Shutdown(ctx)
}

// OtelTracer wraps an OpenTelemetry trace.Tracer to satisfy the observability.Tracer interface.
type OtelTracer struct {
	tracer trace.Tracer
}

// NewOtelTracer creates an observability.Tracer backed by OpenTelemetry.
func NewOtelTracer(tracer trace.Tracer) *OtelTracer {
	return &OtelTracer{tracer: tracer}
}

func (o *OtelTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	ctx, span := o.tracer.Start(ctx, name)
	return ctx, &otelSpan{span: span}
}

type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) End() {
	s.span.End()
}

func (s *otelSpan) RecordError(err error) {
	s.span.RecordError(err)
}

// SetAttributes bridges observability.SpanAttr values into real OTel span
// attributes, so the rich attributes extracted by TracingMiddlewareAdapter
// flow into a production tracer.
func (s *otelSpan) SetAttributes(attrs ...SpanAttr) {
	otAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		otAttrs = append(otAttrs, toOtelAttr(a))
	}
	s.span.SetAttributes(otAttrs...)
}

func toOtelAttr(a SpanAttr) attribute.KeyValue {
	switch v := a.Value.(type) {
	case string:
		return attribute.String(a.Key, v)
	case int:
		return attribute.Int(a.Key, v)
	case int64:
		return attribute.Int64(a.Key, v)
	case float64:
		return attribute.Float64(a.Key, v)
	case bool:
		return attribute.Bool(a.Key, v)
	default:
		return attribute.String(a.Key, fmt.Sprintf("%v", v))
	}
}

// SetAgentAttributes adds common agent attributes to a span.
func SetAgentAttributes(span trace.Span, agentName string) {
	span.SetAttributes(attribute.String("agent.name", agentName))
}

// SetEventAttributes adds event-type attributes to a span.
func SetEventAttributes(span trace.Span, eventType string) {
	span.SetAttributes(attribute.String("event.type", eventType))
}
