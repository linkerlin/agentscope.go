package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockRunner struct {
	resp *Message
	err  error
}

func (m *mockRunner) Run(ctx context.Context, msg *Message) (*Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &Message{Role: "agent", Content: "pong"}, nil
}

func TestServer_AgentCard(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent.json", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got AgentCard
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != card.Name {
		t.Fatalf("expected name %s, got %s", card.Name, got.Name)
	}
}

func TestServer_TaskSendAndGet(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{resp: &Message{Role: "agent", Content: "hello"}}, nil)

	taskID := NewTaskID()
	payload := TaskUpdateRequest{ID: taskID, Message: &Message{Role: "user", Content: "hi"}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/task/send", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	var task Task
	if err := json.Unmarshal(rr.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.ID != taskID {
		t.Fatalf("expected task id %s, got %s", taskID, task.ID)
	}
	if task.Status != TaskStatusWorking {
		t.Fatalf("expected status working, got %s", task.Status)
	}

	// GET task
	req2 := httptest.NewRequest(http.MethodGet, "/task/"+taskID, nil)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestServer_TaskGet_NotFound(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/task/nonexistent", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

type mockStreamingRunner struct {
	resp  *Message
	err   error
	stream []string
}

func (m *mockStreamingRunner) Run(ctx context.Context, msg *Message) (*Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &Message{Role: "agent", Content: "pong"}, nil
}

func (m *mockStreamingRunner) RunStream(ctx context.Context, msg *Message) (<-chan *Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *Message, len(m.stream)+1)
	for _, s := range m.stream {
		ch <- &Message{Role: "agent", Content: s}
	}
	close(ch)
	return ch, nil
}

func TestServer_TaskSendSubscribe_Success(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	runner := &mockStreamingRunner{stream: []string{"hello", " world"}}
	srv := NewServer(card, runner, nil)

	taskID := NewTaskID()
	payload := TaskUpdateRequest{ID: taskID, Message: &Message{Role: "user", Content: "hi"}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/task/sendSubscribe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}
	var task Task
	found := false
	for _, line := range strings.Split(rr.Body.String(), "\n") {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(data), &task); err != nil {
				t.Fatal(err)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no data line found in SSE response")
	}
	if task.Status != TaskStatusWorking {
		t.Fatalf("expected status working, got %s", task.Status)
	}
}

func TestServer_TaskSendSubscribe_NotSupported(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	taskID := NewTaskID()
	payload := TaskUpdateRequest{ID: taskID, Message: &Message{Role: "user", Content: "hi"}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/task/sendSubscribe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rr.Code)
	}
}

func TestServer_TaskSendSubscribe_StreamError(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	runner := &mockStreamingRunner{err: errors.New("stream failed")}
	srv := NewServer(card, runner, nil)

	taskID := NewTaskID()
	payload := TaskUpdateRequest{ID: taskID, Message: &Message{Role: "user", Content: "hi"}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/task/sendSubscribe", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// Allow async goroutine to update the store.
	time.Sleep(50 * time.Millisecond)

	task, err := srv.store.Get(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskStatusFailed {
		t.Fatalf("expected status failed, got %s", task.Status)
	}
}


func TestServer_Addr(t *testing.T) {
	srv := NewServer(AgentCard{Name: "A", URL: "http://localhost:9090"}, &mockRunner{}, nil)
	if srv.Addr() != "http://localhost:9090" {
		t.Fatalf("expected URL as addr, got %s", srv.Addr())
	}
	srv2 := NewServer(AgentCard{Name: "A"}, &mockRunner{}, nil)
	if srv2.Addr() != ":8080" {
		t.Fatalf("expected default addr, got %s", srv2.Addr())
	}
}

func TestServer_TaskCancel(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	taskID := NewTaskID()
	task := &Task{ID: taskID, Status: TaskStatusWorking, Messages: []Message{}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = srv.store.Save(task)

	payload := map[string]string{"id": taskID}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/task/cancel", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got Task
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusCanceled {
		t.Fatalf("expected status canceled, got %s", got.Status)
	}
}

func TestServer_TaskCancel_NotFound(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	payload := map[string]string{"id": "nonexistent"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/task/cancel", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServer_TaskCancel_MethodNotAllowed(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/task/cancel", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestServer_TaskSend_MethodNotAllowed(t *testing.T) {
	card := AgentCard{Name: "TestAgent", URL: "http://localhost:8080"}
	srv := NewServer(card, &mockRunner{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/task/send", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
