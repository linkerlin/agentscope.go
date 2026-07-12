package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/logging"
	"github.com/linkerlin/agentscope.go/service"
)

// threadSafeBuffer is a concurrency-safe bytes.Buffer for capturing logs.
type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newThreadSafeBuffer() *threadSafeBuffer { return &threadSafeBuffer{} }

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureSetupLogging replaces the package default logger with one writing
// JSON to buf, so tests can assert on log output. Restored on cleanup.
func captureSetupLogging(buf *threadSafeBuffer) {
	l := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logging.SetDefault(l)
}

func TestRequestLoggingMiddleware_LogsRequest(t *testing.T) {
	buf := newThreadSafeBuffer()
	captureSetupLogging(buf)

	h := RequestLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logging.FromContext(r.Context()) == nil {
			t.Fatal("logger not in context")
		}
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/api/v1/agents", nil)
	ctx := context.WithValue(req.Context(), service.ContextKeyUserID, "u-log")
	req = req.WithContext(ctx)
	h.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, `"msg":"request"`) {
		t.Fatalf("expected request log, got: %s", out)
	}
	if !strings.Contains(out, `"method":"POST"`) || !strings.Contains(out, `"path":"/api/v1/agents"`) {
		t.Fatalf("missing method/path: %s", out)
	}
	if !strings.Contains(out, `"status":201`) {
		t.Fatalf("missing status: %s", out)
	}
	var parsed map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.Contains(line, `"msg":"request"`) {
			json.Unmarshal([]byte(line), &parsed)
			break
		}
	}
	if parsed["user_id"] != "u-log" {
		t.Fatalf("user_id not logged: %v", parsed["user_id"])
	}
}

func TestRequestLoggingMiddleware_GeneratesRequestID(t *testing.T) {
	buf := newThreadSafeBuffer()
	captureSetupLogging(buf)

	h := RequestLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/health", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"request_id":"req-`) {
		t.Fatalf("expected generated request id: %s", buf.String())
	}
}

func TestRequestLoggingMiddleware_UsesHeaderRequestID(t *testing.T) {
	buf := newThreadSafeBuffer()
	captureSetupLogging(buf)

	h := RequestLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Request-ID", "trace-abc")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"request_id":"trace-abc"`) {
		t.Fatalf("expected header request id: %s", buf.String())
	}
}
