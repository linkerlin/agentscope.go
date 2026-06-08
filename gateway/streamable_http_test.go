package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

func streamableAcceptHeader() string {
	return "application/json, text/event-stream"
}

func TestServer_StreamableHTTP_POST(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.RegisterV2Routes()

	body, _ := json.Marshal(v2ChatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(body))
	req.Header.Set("Accept", streamableAcceptHeader())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "hello") || !strings.Contains(respBody, "done") {
		t.Fatalf("unexpected stream body:\n%s", respBody)
	}
}

func TestServer_StreamableHTTP_POST_RequiresAccept(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.RegisterV2Routes()

	body, _ := json.Marshal(v2ChatRequest{Text: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/v2/chat", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d", rec.Code)
	}
}

func TestServer_StreamableHTTP_GET_Subscribe(t *testing.T) {
	srv := NewServer(&slowMockV2Agent{delay: 20 * time.Millisecond})
	sm := NewSessionManager()
	srv.WithSessionManager(sm)
	srv.RegisterV2Routes()

	// Start a run in background via POST (legacy path, no strict Accept).
	startBody, _ := json.Marshal(v2ChatRequest{Text: "hi", SessionID: "sess-1"})
	startReq := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(startBody))
	startRec := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.ServeHTTP(startRec, startReq)
	}()

	time.Sleep(5 * time.Millisecond)

	getReq := httptest.NewRequest(http.MethodGet, "/v2/chat?session_id=sess-1", nil)
	getReq.Header.Set("Accept", streamableAcceptHeader())
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	wg.Wait()

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", getRec.Code)
	}
	if sid := getRec.Header().Get(HeaderAgentSessionID); sid != "sess-1" {
		t.Fatalf("expected Agent-Session-Id sess-1, got %q", sid)
	}
	if !strings.Contains(getRec.Body.String(), "hello") {
		t.Fatalf("GET subscribe should replay events, got:\n%s", getRec.Body.String())
	}
}

// slowMockV2Agent delays before emitting events so Subscribe can join mid-run.
type slowMockV2Agent struct {
	delay time.Duration
}

func (m *slowMockV2Agent) Name() string { return "slow-mock" }

func (m *slowMockV2Agent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(), nil
}

func (m *slowMockV2Agent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 1)
	ch <- message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build()
	close(ch)
	return ch, nil
}

func (m *slowMockV2Agent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return m.Call(ctx, msg)
}

func (m *slowMockV2Agent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 3)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				return
			}
		}
		select {
		case ch <- event.NewTextBlockDelta("r1", 0, "hello"):
		case <-ctx.Done():
			return
		}
		select {
		case ch <- event.NewReplyEnd("r1", ""):
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func (m *slowMockV2Agent) LoadState(state *agent.AgentState) error { return nil }
func (m *slowMockV2Agent) SaveState() (*agent.AgentState, error)   { return nil, nil }
func (m *slowMockV2Agent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	return nil
}

func TestServer_StreamableHTTP_DELETE(t *testing.T) {
	srv := NewServer(&slowMockV2Agent{delay: 2 * time.Second})
	sm := NewSessionManager()
	srv.WithSessionManager(sm)
	srv.RegisterV2Routes()

	startBody, _ := json.Marshal(v2ChatRequest{Text: "hi", SessionID: "sess-del"})
	startReq := httptest.NewRequest(http.MethodPost, "/v2/chat/stream", bytes.NewReader(startBody))
	startRec := httptest.NewRecorder()

	go srv.ServeHTTP(startRec, startReq)
	time.Sleep(20 * time.Millisecond)

	if !sm.IsActive("sess-del") {
		t.Fatal("expected active run before DELETE")
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/v2/chat?session_id=sess-del", nil)
	delRec := httptest.NewRecorder()
	srv.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE expected 204, got %d body=%s", delRec.Code, delRec.Body.String())
	}
	if sid := delRec.Header().Get(HeaderAgentSessionID); sid != "sess-del" {
		t.Fatalf("expected Agent-Session-Id sess-del, got %q", sid)
	}

	deadline := time.After(3 * time.Second)
	for sm.IsActive("sess-del") {
		select {
		case <-deadline:
			t.Fatal("run still active after DELETE")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestServer_StreamableHTTP_DELETE_NotFound(t *testing.T) {
	srv := NewServer(&mockV2Agent{})
	srv.WithSessionManager(NewSessionManager())
	srv.RegisterV2Routes()

	req := httptest.NewRequest(http.MethodDelete, "/v2/chat?session_id=missing", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
