package event

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects lightweight statistics about event flow.
// It is safe for concurrent use.
type MetricsCollector struct {
	mu sync.RWMutex

	publishedTotal uint64
	receivedTotal  uint64

	// EventTypeCounts maps event type to publish count.
	eventTypeCounts map[string]uint64

	// Latency distribution in milliseconds, bucket boundaries: 1, 5, 10, 50, 100, +Inf
	latencyBuckets []uint64 // len=6
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		eventTypeCounts: make(map[string]uint64),
		latencyBuckets:  make([]uint64, 6),
	}
}

// RecordPublished records that an event was published.
func (m *MetricsCollector) RecordPublished(ev AgentEvent) {
	atomic.AddUint64(&m.publishedTotal, 1)
	m.mu.Lock()
	m.eventTypeCounts[ev.EventType()]++
	m.mu.Unlock()
}

// RecordReceived records that an event was received by a subscriber.
func (m *MetricsCollector) RecordReceived() {
	atomic.AddUint64(&m.receivedTotal, 1)
}

// RecordLatency records the end-to-end latency of an event in milliseconds.
func (m *MetricsCollector) RecordLatency(ms int64) {
	idx := latencyBucketIndex(ms)
	atomic.AddUint64(&m.latencyBuckets[idx], 1)
}

// Snapshot returns a point-in-time copy of the metrics.
func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	counts := make(map[string]uint64, len(m.eventTypeCounts))
	for k, v := range m.eventTypeCounts {
		counts[k] = v
	}
	m.mu.RUnlock()

	buckets := make([]uint64, len(m.latencyBuckets))
	for i := range m.latencyBuckets {
		buckets[i] = atomic.LoadUint64(&m.latencyBuckets[i])
	}

	return MetricsSnapshot{
		PublishedTotal:  atomic.LoadUint64(&m.publishedTotal),
		ReceivedTotal:   atomic.LoadUint64(&m.receivedTotal),
		EventTypeCounts: counts,
		LatencyBuckets:  buckets,
	}
}

// MetricsSnapshot is a point-in-time snapshot of event metrics.
type MetricsSnapshot struct {
	PublishedTotal  uint64            `json:"published_total"`
	ReceivedTotal   uint64            `json:"received_total"`
	EventTypeCounts map[string]uint64 `json:"event_type_counts"`
	LatencyBuckets  []uint64          `json:"latency_buckets"`
}

// latencyBucketIndex returns the bucket index for a latency in milliseconds.
func latencyBucketIndex(ms int64) int {
	switch {
	case ms <= 1:
		return 0
	case ms <= 5:
		return 1
	case ms <= 10:
		return 2
	case ms <= 50:
		return 3
	case ms <= 100:
		return 4
	default:
		return 5
	}
}

// BusWithMetrics wraps a Bus and records metrics for every event.
type BusWithMetrics struct {
	*Bus
	metrics *MetricsCollector
}

// NewBusWithMetrics creates an event bus with metrics collection.
func NewBusWithMetrics() *BusWithMetrics {
	return &BusWithMetrics{
		Bus:     NewBus(64),
		metrics: NewMetricsCollector(),
	}
}

// Metrics returns the underlying metrics collector.
func (b *BusWithMetrics) Metrics() *MetricsCollector {
	return b.metrics
}

// Publish publishes an event and records metrics.
func (b *BusWithMetrics) Publish(ev AgentEvent) {
	b.metrics.RecordPublished(ev)
	b.Bus.Publish(ev)
}

// PublishWithLatency publishes an event and records its latency.
func (b *BusWithMetrics) PublishWithLatency(ev AgentEvent, d time.Duration) {
	b.metrics.RecordPublished(ev)
	b.metrics.RecordLatency(int64(d / time.Millisecond))
	b.Bus.Publish(ev)
}

// MetricsHandler returns an http.HandlerFunc that serves the current metrics
// snapshot as JSON. It can be mounted on any HTTP mux (e.g. /metrics/events).
func MetricsHandler(collector *MetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		snap := collector.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap)
	}
}
