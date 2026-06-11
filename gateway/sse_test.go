package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// mockV2Agent is a minimal V2Agent for testing.
type mockV2Agent struct{}

func (m *mockV2Agent) Name() string { return "mock-v2" }

func (m *mockV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *mockV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

func (m *mockV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}
func (m *mockV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 3)
	ch <- event.NewTextBlockDelta("r1", 0, "hello")
	ch <- event.NewTextBlockDelta("r1", 0, " world")
	ch <- event.NewReplyEnd("r1", "")
	close(ch)
	return ch, nil
}

func (m *mockV2Agent) LoadState(state *agent.AgentState) error { return nil }
func (m *mockV2Agent) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *mockV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func TestServer_V2ChatStream(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.RegisterV2Routes()

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "hello") {
		t.Fatalf("expected 'hello' in SSE stream, got:\n%s", respBody)
	}
	if !strings.Contains(respBody, "done") {
		t.Fatalf("expected terminal 'done' event, got:\n%s", respBody)
	}
}

func TestServer_V2ChatStream_AGUI(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.RegisterV2Routes()

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream?protocol=agui", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "TEXT_MESSAGE_CONTENT") {
		t.Fatalf("expected AG-UI event in stream, got:\n%s", respBody)
	}
	if !strings.Contains(respBody, "STREAM_DONE") {
		t.Fatalf("expected STREAM_DONE terminal event, got:\n%s", respBody)
	}
	if strings.Contains(respBody, `"event_type":"done"`) {
		t.Fatalf("native done event should not appear in AG-UI mode, got:\n%s", respBody)
	}
}

func TestServer_V2ChatStream_NotV2Agent(t *testing.T) {
	// Use a plain agent that does NOT implement V2Agent.
	plain := &mockPlainAgent{}
	srv := NewServer(plain)
	srv.RegisterV2Routes()

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestServer_V2ChatStream_MethodNotAllowed(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.RegisterV2Routes()

	req := httptest.NewRequest(http.MethodGet, "/v2/chat/stream", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

type mockPlainAgent struct{}

func (m *mockPlainAgent) Name() string { return "plain" }
func (m *mockPlainAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}
func (m *mockPlainAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

func TestServer_V2ChatStream_MissingText(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.RegisterV2Routes()

	body, _ := json.Marshal(chatRequest{Text: ""})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

type mockV2AgentError struct{}

func (m *mockV2AgentError) Name() string { return "mock-v2-err" }
func (m *mockV2AgentError) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, errors.New("agent error")
}
func (m *mockV2AgentError) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	return nil, errors.New("agent error")
}
func (m *mockV2AgentError) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return nil, errors.New("reply error")
}
func (m *mockV2AgentError) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	return nil, errors.New("stream error")
}
func (m *mockV2AgentError) LoadState(state *agent.AgentState) error { return nil }
func (m *mockV2AgentError) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *mockV2AgentError) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

// mockV2AgentWithSuspend simulates an agent that suspends on a tool call.
type mockV2AgentWithSuspend struct {
	mu       sync.Mutex
	injectCh chan event.AgentEvent
	state    *agent.AgentState
}

func newMockV2AgentWithSuspend() *mockV2AgentWithSuspend {
	return &mockV2AgentWithSuspend{injectCh: make(chan event.AgentEvent, 1)}
}

func (m *mockV2AgentWithSuspend) Name() string { return "mock-v2-suspend" }
func (m *mockV2AgentWithSuspend) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}
func (m *mockV2AgentWithSuspend) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}
func (m *mockV2AgentWithSuspend) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}
func (m *mockV2AgentWithSuspend) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 4)
	go func() {
		defer close(ch)
		ch <- event.NewTextBlockDelta("r1", 0, "before")
		ch <- event.NewRequireUserConfirm("r1", "c1", []event.ToolCallSummary{
			{ID: "tc1", Name: "tool", Input: map[string]any{}},
		})
		// Wait for inject
		select {
		case <-m.injectCh:
		case <-ctx.Done():
			return
		}
		ch <- event.NewTextBlockDelta("r1", 0, "after")
		ch <- event.NewReplyEnd("r1", "")
	}()
	return ch, nil
}
func (m *mockV2AgentWithSuspend) LoadState(state *agent.AgentState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	return nil
}
func (m *mockV2AgentWithSuspend) SaveState() (*agent.AgentState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &agent.AgentState{ReplyID: "r1", AgentName: m.Name()}, nil
}
func (m *mockV2AgentWithSuspend) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	select {
	case m.injectCh <- ev:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestServer_V2ChatStream_AgentError(t *testing.T) {
	srv := NewServer(&mockV2AgentError{})
	srv.RegisterV2Routes()

	body, _ := json.Marshal(chatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
