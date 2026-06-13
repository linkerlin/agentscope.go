package gateway

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// MetricsCollector holds runtime and application metrics for Prometheus exposition.
type MetricsCollector struct {
	startTime     time.Time
	requestCount  int64
	requestErrors int64
	activeSessions int64
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{startTime: time.Now()}
}

// RecordRequest increments the request counter.
func (m *MetricsCollector) RecordRequest() {
	m.requestCount++
}

// RecordError increments the error counter.
func (m *MetricsCollector) RecordError() {
	m.requestErrors++
}

// RecordActiveSessions sets the active sessions count.
func (m *MetricsCollector) RecordActiveSessions(n int64) {
	m.activeSessions = n
}

// PrometheusHandler exposes metrics in Prometheus text format.
func (m *MetricsCollector) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		uptime := time.Since(m.startTime).Seconds()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "# AgentScope.Go Prometheus Metrics\n\n")

		// Go runtime metrics
		fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(w, "go_goroutines %d\n\n", runtime.NumGoroutine())

		fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n\n", memStats.Alloc)

		fmt.Fprintf(w, "# TYPE go_memstats_sys_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_sys_bytes %d\n\n", memStats.Sys)

		fmt.Fprintf(w, "# TYPE go_memstats_heap_alloc_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n\n", memStats.HeapAlloc)

		fmt.Fprintf(w, "# TYPE go_memstats_heap_inuse_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n\n", memStats.HeapInuse)

		fmt.Fprintf(w, "# TYPE go_memstats_heap_objects gauge\n")
		fmt.Fprintf(w, "go_memstats_heap_objects %d\n\n", memStats.HeapObjects)

		fmt.Fprintf(w, "# TYPE go_gc_duration_seconds summary\n")
		fmt.Fprintf(w, "go_gc_duration_seconds %d\n\n", memStats.NumGC)

		// Application metrics
		fmt.Fprintf(w, "# TYPE agentscope_uptime_seconds gauge\n")
		fmt.Fprintf(w, "agentscope_uptime_seconds %.3f\n\n", uptime)

		fmt.Fprintf(w, "# TYPE agentscope_requests_total counter\n")
		fmt.Fprintf(w, "agentscope_requests_total %d\n\n", m.requestCount)

		fmt.Fprintf(w, "# TYPE agentscope_request_errors_total counter\n")
		fmt.Fprintf(w, "agentscope_request_errors_total %d\n\n", m.requestErrors)

		fmt.Fprintf(w, "# TYPE agentscope_active_sessions gauge\n")
		fmt.Fprintf(w, "agentscope_active_sessions %d\n\n", m.activeSessions)

		fmt.Fprintf(w, "# TYPE agentscope_version info\n")
		fmt.Fprintf(w, "agentscope_version{version=\"%s\"} 1\n\n", "2.0.0")
	}
}

// MetricsMiddleware wraps an HTTP handler to record request metrics.
func MetricsMiddleware(metrics *MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metrics.RecordRequest()
			next.ServeHTTP(w, r)
		})
	}
}
