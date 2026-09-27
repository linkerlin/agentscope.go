package gateway

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OTelMiddleware wraps an HTTP handler with OpenTelemetry tracing and metrics.
// It records request latency, count, and status code.
type OTelMiddleware struct {
	tracer      trace.Tracer
	reqCounter  metric.Int64Counter
	reqDuration metric.Float64Histogram
}

// NewOTelMiddleware creates a new OpenTelemetry middleware.
func NewOTelMiddleware(tracer trace.Tracer, meter metric.Meter) (*OTelMiddleware, error) {
	reqCounter, err := meter.Int64Counter("http.requests.total",
		metric.WithDescription("Total HTTP requests"))
	if err != nil {
		return nil, err
	}
	reqDuration, err := meter.Float64Histogram("http.request.duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"))
	if err != nil {
		return nil, err
	}
	return &OTelMiddleware{
		tracer:      tracer,
		reqCounter:  reqCounter,
		reqDuration: reqDuration,
	}, nil
}

// Wrap returns an HTTP handler wrapped with tracing and metrics.
func (m *OTelMiddleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, span := m.tracer.Start(r.Context(), "http.request")
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
			attribute.String("http.target", r.URL.String()),
		)

		// Wrap response writer to capture status code.
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next(rec, r.WithContext(ctx))

		duration := time.Since(start).Seconds()
		span.SetAttributes(attribute.Int("http.status_code", rec.statusCode))

		attrs := []attribute.KeyValue{
			attribute.String("method", r.Method),
			attribute.String("route", r.URL.Path),
			attribute.Int("status", rec.statusCode),
		}
		m.reqCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.reqDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
// It is shared by the OTel, audit, and request-logging middlewares, so it
// must stay transparent for the optional interfaces streaming endpoints
// depend on (22.3): http.Flusher for SSE incremental delivery and
// http.Hijacker for the WebSocket upgrade. Without these forwards, a
// wrapped server would break SSE (handlers abort with "streaming not
// supported") and every WebSocket upgrade.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the wrapped writer when it supports flushing, keeping
// SSE incremental delivery working through the recorder.
func (rec *statusRecorder) Flush() {
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the wrapped writer when it supports hijacking, keeping
// the WebSocket upgrade working through the recorder.
func (rec *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rec.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("statusRecorder: wrapped ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Unwrap lets http.NewResponseController reach the underlying writer's
// optional interfaces through the standard unwrapping mechanism.
func (rec *statusRecorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

// WithOTelTracing wraps the gateway server with OpenTelemetry tracing on all routes.
func (s *Server) WithOTelTracing(tracer trace.Tracer, meter metric.Meter) error {
	otel, err := NewOTelMiddleware(tracer, meter)
	if err != nil {
		return err
	}
	// Wrap the entire mux so every request gets traced without re-registering routes.
	s.otelHandler = otel.Wrap(s.mux.ServeHTTP)
	return nil
}
