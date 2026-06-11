package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/service"
)

type mockAgent struct {
	name   string
	resp   *message.Msg
	err    error
	stream []*message.Msg
}

func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return message.NewMsg().Role(message.RoleAssistant).TextContent("pong").Build(), nil
}

func (m *mockAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *message.Msg, len(m.stream)+1)
	for _, msg := range m.stream {
		ch <- msg
	}
	close(ch)
	return ch, nil
}

var _ agent.Agent = (*mockAgent)(nil)

func TestGateway_Chat_Success(t *testing.T) {
	a := &mockAgent{name: "test", resp: message.NewMsg().Role(message.RoleAssistant).TextContent("hello").Build()}
	srv := NewServer(a)

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["content"] != "hello" {
		t.Fatalf("expected 'hello', got %q", resp["content"])
	}
}

func TestGateway_Chat_MissingText(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	body, _ := json.Marshal(chatRequest{Text: ""})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGateway_Chat_AgentError(t *testing.T) {
	a := &mockAgent{name: "test", err: errors.New("boom")}
	srv := NewServer(a)

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestGateway_ChatStream_Success(t *testing.T) {
	stream := []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("he").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("llo").Build(),
	}
	a := &mockAgent{name: "test", stream: stream}
	srv := NewServer(a)

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

	lines := strings.Split(rr.Body.String(), "\n")
	var deltas []string
	var done bool
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			var ev streamEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
				continue
			}
			if ev.Done {
				done = true
			} else {
				deltas = append(deltas, ev.Delta)
			}
		}
	}
	got := strings.Join(deltas, "")
	if got != "hello" {
		t.Fatalf("expected stream 'hello', got %q", got)
	}
	if !done {
		t.Fatal("expected final done event")
	}
}

func TestGateway_ChatStream_MethodNotAllowed(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	req := httptest.NewRequest(http.MethodGet, "/chat/stream", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestGateway_Chat_MethodNotAllowed(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestGateway_SessionCount(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	if srv.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions initially, got %d", srv.SessionCount())
	}
}

func TestGateway_HealthCheck(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["status"] != "healthy" {
		t.Fatalf("expected healthy, got %v", status["status"])
	}
}

func TestGateway_HealthCheck_WithSessionManager(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"}).
		WithSessionManager(NewSessionManager())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if _, ok := status["active_sessions"]; !ok {
		t.Fatal("expected active_sessions in health with session manager")
	}
}

func TestGateway_HealthCheck_WithAuth(t *testing.T) {
	storage := service.NewMemoryStorage()
	apiAuth := service.NewAPIKeyAuthenticator(storage, "X-API-Key")
	srv := NewServer(&mockAgent{name: "test"}).
		WithStorage(storage).
		WithAuthenticator(apiAuth)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["auth"] != "enabled" {
		t.Fatalf("expected auth enabled, got %v", status["auth"])
	}
	if status["storage"] != "configured" {
		t.Fatalf("expected storage configured, got %v", status["storage"])
	}
}

func TestGateway_RequestID(t *testing.T) {
	srv := NewServer(&mockAgent{name: "test"})

	// Request without header gets auto-generated ID.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	generated := rec.Header().Get("X-Request-ID")
	if generated == "" {
		t.Fatal("expected auto-generated X-Request-ID header")
	}

	// Request with explicit header preserves it.
	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	req2.Header.Set("X-Request-ID", "my-custom-id")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	got := rec2.Header().Get("X-Request-ID")
	if got != "my-custom-id" {
		t.Fatalf("expected my-custom-id, got %q", got)
	}
}
