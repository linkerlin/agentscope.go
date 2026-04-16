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
