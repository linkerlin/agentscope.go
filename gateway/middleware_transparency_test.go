package gateway

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/linkerlin/agentscope.go/service"
)

// fakeFlushHijackWriter is a ResponseWriter that implements Flusher and
// Hijacker, recording whether each optional interface was reached.
type fakeFlushHijackWriter struct {
	httptest.ResponseRecorder
	flushes  int
	hijacked bool
}

func (f *fakeFlushHijackWriter) Flush() { f.flushes++ }
func (f *fakeFlushHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	f.hijacked = true
	// Return a closed pipe pair; the caller in these tests only checks that
	// the forward happened.
	c1, _ := net.Pipe()
	_ = c1.Close()
	return c1, bufio.NewReadWriter(bufio.NewReader(c1), bufio.NewWriter(c1)), nil
}

// TestStatusRecorder_ForwardsOptionalInterfaces locks the 22.3 rule: the
// shared recorder exposes Flusher and Hijacker through the wrap, and
// ResponseController can unwrap through it.
func TestStatusRecorder_ForwardsOptionalInterfaces(t *testing.T) {
	inner := &fakeFlushHijackWriter{}
	var w http.ResponseWriter = &statusRecorder{ResponseWriter: inner, statusCode: http.StatusOK}

	f, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("recorder does not implement http.Flusher")
	}
	f.Flush()
	if inner.flushes != 1 {
		t.Fatalf("Flush not forwarded: %d", inner.flushes)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("recorder does not implement http.Hijacker")
	}
	if _, _, err := hj.Hijack(); err != nil {
		t.Fatalf("Hijack forward failed: %v", err)
	}
	if !inner.hijacked {
		t.Fatal("Hijack not forwarded to wrapped writer")
	}

	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		t.Fatalf("ResponseController.Flush through recorder: %v", err)
	}
	if inner.flushes != 2 {
		t.Fatalf("ResponseController flush not forwarded: %d", inner.flushes)
	}
}

// otelAuditServer builds a V2 server with BOTH the OTel mux wrapper and the
// audit middleware active (the composition that used to break streaming).
func otelAuditServer(t *testing.T) *httptest.Server {
	t.Helper()
	storage := service.NewMemoryStorage()
	sm := NewSessionManager()
	srv := NewServer(&mockV2Agent{}).
		WithStorage(storage).
		WithSessionManager(sm).
		WithAuditLogger(service.NewMemoryAuditLogger())
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(nil) }()
	mp := sdkmetric.NewMeterProvider()
	defer func() { _ = mp.Shutdown(nil) }()
	if err := srv.WithOTelTracing(tp.Tracer("test"), mp.Meter("test")); err != nil {
		t.Fatal(err)
	}
	srv.RegisterV2Routes()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

// TestMiddleware_SSEIncrementalThroughWrappers locks the 22.3 rule: with
// OTel + audit active, the SSE endpoint streams incrementally — each event
// is flushed to the client before the run completes (previously the Flusher
// assertion failed and the handler aborted with 500).
func TestMiddleware_SSEIncrementalThroughWrappers(t *testing.T) {
	ts := otelAuditServer(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v2/chat/stream", strings.NewReader(`{"text":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 through wrappers, got %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", ct)
	}

	// Read the first SSE event with a deadline: incremental delivery means
	// data arrives while the stream is still open (the response body has not
	// hit EOF yet).
	br := bufio.NewReader(resp.Body)
	errCh := make(chan error, 1)
	first := make(chan string, 1)
	go func() {
		line, err := br.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		first <- line
	}()
	select {
	case line := <-first:
		if !strings.HasPrefix(line, "data:") {
			t.Fatalf("expected SSE data line, got %q", line)
		}
	case err := <-errCh:
		t.Fatalf("reading first SSE event: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("no SSE event within 5s: flushing is broken through the middleware wrappers")
	}
}

// TestMiddleware_WebSocketUpgradeThroughWrappers locks the 22.3 rule: with
// OTel + audit active, the WebSocket handshake completes with 101 (the
// upgrade needs the Hijacker interface through the recorder).
func TestMiddleware_WebSocketUpgradeThroughWrappers(t *testing.T) {
	ts := otelAuditServer(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v2/chat/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
			DisableKeepAlives: true,
		},
		// A 101 response ends the HTTP conversation; do not follow anything.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 101 through wrappers, got %d: %s", resp.StatusCode, body)
	}
	if resp.Header.Get("Upgrade") != "websocket" {
		t.Fatal("missing websocket upgrade header in response")
	}
}
