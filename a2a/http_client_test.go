package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/send" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(Task{
			ID:     "task-1",
			Status: TaskStatusCompleted,
			Messages: []Message{
				{Role: "user", Content: "hi"},
				{Role: "agent", Content: "hello"},
			},
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	resp, err := client.Send(context.Background(), &Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Fatalf("expected 'hello', got %q", resp.Content)
	}
}

func TestHTTPClient_Send_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	_, err := client.Send(context.Background(), &Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClient_SendSubscribe_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/sendSubscribe" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusAccepted)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// First event: task snapshot
		task := Task{ID: "task-2", Status: TaskStatusWorking, Messages: []Message{{Role: "user", Content: "hi"}}}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", mustJSON(task))
		flusher.Flush()

		// Second event: agent partial reply
		task.Messages = append(task.Messages, Message{Role: "agent", Content: "partial"})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", mustJSON(task))
		flusher.Flush()

		_, _ = fmt.Fprintln(w, "data: [DONE]")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ch, err := client.SendSubscribe(context.Background(), &Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}

	var contents []string
	for msg := range ch {
		if msg != nil {
			contents = append(contents, msg.Content)
		}
	}
	if len(contents) != 2 || contents[0] != "hi" || contents[1] != "partial" {
		t.Fatalf("unexpected stream contents: %v", contents)
	}
}

func TestHTTPClient_SendSubscribe_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not_impl"}`, http.StatusNotImplemented)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	_, err := client.SendSubscribe(context.Background(), &Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClient_SendSubscribe_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusAccepted)
		flusher, _ := w.(http.Flusher)
		task := Task{ID: "task-3", Status: TaskStatusWorking, Messages: []Message{{Role: "user", Content: "hi"}}}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", mustJSON(task))
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := NewHTTPClient(server.URL)
	ch, err := client.SendSubscribe(ctx, &Message{Role: "user", Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	// Consume the first event then cancel.
	<-ch
	cancel()
	// Channel should close shortly after context cancellation.
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}


func TestHTTPClient_Send_NoMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(Task{ID: "task-x", Status: TaskStatusCompleted, Messages: []Message{}})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	_, err := client.Send(context.Background(), &Message{Role: "user", Content: "hi"})
	if err == nil {
		t.Fatal("expected error when no messages in task response")
	}
}

func TestHTTPClient_WaitForTask(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/task-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		callCount++
		status := TaskStatusWorking
		if callCount >= 3 {
			status = TaskStatusCompleted
		}
		_ = json.NewEncoder(w).Encode(Task{ID: "task-1", Status: status, Messages: []Message{{Role: "user", Content: "hi"}}})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	task, err := client.WaitForTask(context.Background(), "task-1", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskStatusCompleted {
		t.Fatalf("expected completed, got %s", task.Status)
	}
	if callCount < 3 {
		t.Fatalf("expected at least 3 polls, got %d", callCount)
	}
}

func TestHTTPClient_WaitForTask_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Task{ID: "task-1", Status: TaskStatusWorking})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := NewHTTPClient(server.URL)
	_, err := client.WaitForTask(ctx, "task-1", 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from context")
	}
}

func TestHTTPClient_CancelTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/cancel" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(Task{ID: req["id"], Status: TaskStatusCanceled})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	task, err := client.CancelTask(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskStatusCanceled {
		t.Fatalf("expected canceled, got %s", task.Status)
	}
}

func TestHTTPClient_CancelTask_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	_, err := client.CancelTask(context.Background(), "task-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClient_Close(t *testing.T) {
	client := NewHTTPClient("http://localhost:8080")
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}
