// Package otelbridge provides an optional bridge to OpenTelemetry.
//
// Import this package with a blank import to automatically wire OTel
// trace context extraction into agentscope.go's observability layer:
//
//	import _ "github.com/linkerlin/agentscope.go/observability/otelbridge"
package otelbridge

import (
	"context"

	"github.com/linkerlin/agentscope.go/observability"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	observability.TraceContextFromContext = func(ctx context.Context) observability.TraceContext {
		span := trace.SpanFromContext(ctx)
		if !span.IsRecording() && !span.SpanContext().IsValid() {
			// fallback: try SpanContext even if not recording
		}
		sc := span.SpanContext()
		if !sc.IsValid() {
			return observability.TraceContext{}
		}
		return observability.TraceContext{
			TraceID: sc.TraceID().String(),
			SpanID:  sc.SpanID().String(),
		}
	}
}
