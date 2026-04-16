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
