package observability

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/event"
)

func TestLangfuseClient_IngestFormat(t *testing.T) {
	var got struct {
		Auth string
		Body map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/ingestion" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		got.Auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewLangfuseClient("pk-test", "sk-test").WithBaseURL(srv.URL)
	err := c.Ingest(context.Background(), []LangfuseEvent{
		NewLangfuseEvent(LangfuseTraceCreate, map[string]any{"id": "t1", "name": "reply"}),
		NewLangfuseEvent(LangfuseGenerationCreate, map[string]any{"id": "g1", "traceId": "t1"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auth == "" || got.Auth[:6] != "Basic " {
		t.Fatalf("expected Basic auth, got %q", got.Auth)
	}
	batch, ok := got.Body["batch"].([]any)
	if !ok || len(batch) != 2 {
		t.Fatalf("expected 2 events in batch, got %v", got.Body)
	}
	first := batch[0].(map[string]any)
	if first["type"] != "trace-create" {
		t.Fatalf("first event type = %v", first["type"])
	}
}

func TestLangfuseClient_IngestEmpty(t *testing.T) {
	c := NewLangfuseClient("pk", "sk")
	if err := c.Ingest(context.Background(), nil); err != nil {
		t.Fatal("empty batch should not error")
	}
}

func TestLangfuseClient_IngestMissingKeys(t *testing.T) {
	c := NewLangfuseClient("", "")
	if err := c.Ingest(context.Background(), []LangfuseEvent{{Type: "trace-create"}}); err == nil {
		t.Fatal("expected error for missing keys")
	}
}

func TestLangfuseClient_IngestServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()
	c := NewLangfuseClient("pk", "sk").WithBaseURL(srv.URL)
	err := c.Ingest(context.Background(), []LangfuseEvent{NewLangfuseEvent("trace-create", map[string]any{"id": "x"})})
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestNewLangfuseEvent(t *testing.T) {
	ev := NewLangfuseEvent(LangfuseSpanCreate, map[string]any{"id": "s1"})
	if ev.ID == "" {
		t.Fatal("event id should be set")
	}
	if ev.Type != "span-create" {
		t.Fatalf("type = %s", ev.Type)
	}
	if ev.Timestamp == "" {
		t.Fatal("timestamp should be set")
	}
	if _, err := time.Parse(time.RFC3339Nano, ev.Timestamp); err != nil {
		t.Fatalf("timestamp not RFC3339: %v", err)
	}
}

func TestLangfuseObserver_EventMapping(t *testing.T) {
	var mu sync.Mutex
	var ingested []LangfuseEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Batch []LangfuseEvent `json:"batch"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		ingested = append(ingested, body.Batch...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewLangfuseClient("pk", "sk").WithBaseURL(srv.URL)
	o := NewLangfuseObserver(c, "user-1", "sess-1")

	bus := event.NewBus(64)
	go o.Observe(context.Background(), bus)
	time.Sleep(50 * time.Millisecond) // let observer subscribe

	// Emit a reply lifecycle.
	bus.Publish(event.NewReplyStart("reply-1", "Friday"))
	bus.Publish(event.NewToolCallStart("reply-1", 0, "tc-1", "bash"))
	bus.Publish(event.NewToolCallEnd("reply-1", 0, "tc-1"))
	bus.Publish(event.NewReplyEnd("reply-1", "Friday"))

	time.Sleep(150 * time.Millisecond) // let flush happen

	mu.Lock()
	defer mu.Unlock()
	// Expect: trace-create + generation-create + span-create + span-update + generation-update
	types := map[string]int{}
	for _, ev := range ingested {
		types[ev.Type]++
	}
	if types[LangfuseTraceCreate] < 1 {
		t.Fatalf("expected trace-create, got %+v", types)
	}
	if types[LangfuseGenerationCreate] < 2 {
		t.Fatalf("expected >=2 generation-create (start+end), got %+v", types)
	}
	if types[LangfuseSpanCreate] < 2 {
		t.Fatalf("expected >=2 span-create (start+end), got %+v", types)
	}
	// Verify trace body has session/user
	var traceBody map[string]any
	for _, ev := range ingested {
		if ev.Type == LangfuseTraceCreate {
			traceBody = ev.Body
		}
	}
	if traceBody["sessionId"] != "sess-1" || traceBody["userId"] != "user-1" {
		t.Fatalf("trace body missing session/user: %+v", traceBody)
	}
}
