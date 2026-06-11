package event

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsCollector(t *testing.T) {
	m := NewMetricsCollector()

	// Record events.
	m.RecordPublished(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: "text_delta"}})
	m.RecordPublished(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: "text_delta"}})
	m.RecordPublished(&ThinkingBlockDeltaEvent{baseEvent: baseEvent{EventType_: "thinking_delta"}})
	m.RecordReceived()
	m.RecordReceived()
	m.RecordReceived()
	m.RecordLatency(0)
	m.RecordLatency(3)
	m.RecordLatency(8)
	m.RecordLatency(30)
	m.RecordLatency(80)
	m.RecordLatency(200)

	snap := m.Snapshot()
	if snap.PublishedTotal != 3 {
		t.Fatalf("expected published_total=3, got %d", snap.PublishedTotal)
	}
	if snap.ReceivedTotal != 3 {
		t.Fatalf("expected received_total=3, got %d", snap.ReceivedTotal)
	}
	if snap.EventTypeCounts["text_delta"] != 2 {
		t.Fatalf("expected text_delta=2, got %d", snap.EventTypeCounts["text_delta"])
	}
	if snap.EventTypeCounts["thinking_delta"] != 1 {
		t.Fatalf("expected thinking_delta=1, got %d", snap.EventTypeCounts["thinking_delta"])
	}
	// Latency buckets: [0-1, 1-5, 5-10, 10-50, 50-100, 100+]
	expected := []uint64{1, 1, 1, 1, 1, 1}
	for i, v := range expected {
		if snap.LatencyBuckets[i] != v {
			t.Fatalf("expected latency bucket %d=%d, got %d", i, v, snap.LatencyBuckets[i])
		}
	}
}

func TestBusWithMetrics(t *testing.T) {
	bus := NewBusWithMetrics()
	_, ch, _ := bus.Subscribe()

	ev := &TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: "text_delta"}}
	bus.PublishWithLatency(ev, 5*time.Millisecond)

	// Wait for delivery.
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	snap := bus.Metrics().Snapshot()
	if snap.PublishedTotal != 1 {
		t.Fatalf("expected published_total=1, got %d", snap.PublishedTotal)
	}
	if snap.LatencyBuckets[1] != 1 {
		t.Fatalf("expected latency bucket 1=1, got %d", snap.LatencyBuckets[1])
	}
}

func TestMetricsHandler(t *testing.T) {
	m := NewMetricsCollector()
	m.RecordPublished(&TextBlockDeltaEvent{baseEvent: baseEvent{EventType_: "text_delta"}})
	m.RecordReceived()
	m.RecordLatency(5)

	handler := MetricsHandler(m)
	req := httptest.NewRequest(http.MethodGet, "/metrics/events", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var snap MetricsSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if snap.PublishedTotal != 1 {
		t.Fatalf("expected published_total=1, got %d", snap.PublishedTotal)
	}
	if snap.ReceivedTotal != 1 {
		t.Fatalf("expected received_total=1, got %d", snap.ReceivedTotal)
	}
	if snap.LatencyBuckets[1] != 1 {
		t.Fatalf("expected latency bucket 1=1, got %d", snap.LatencyBuckets[1])
	}
}

func TestMetricsHandler_MethodNotAllowed(t *testing.T) {
	m := NewMetricsCollector()
	handler := MetricsHandler(m)
	req := httptest.NewRequest(http.MethodPost, "/metrics/events", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
