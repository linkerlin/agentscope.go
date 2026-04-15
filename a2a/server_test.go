package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
