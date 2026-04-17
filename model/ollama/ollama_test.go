package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestOllamaBuilder_Formatter(t *testing.T) {
	f := formatter.NewOllamaFormatter()
	m, err := NewBuilder().
		Formatter(f).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if m.ModelName() != "llama3.2" {
		t.Fatalf("unexpected model name: %s", m.ModelName())
	}
}

func TestOllamaBuilder_Default(t *testing.T) {
	m, err := NewBuilder().Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if m.ModelName() != "llama3.2" {
		t.Fatalf("unexpected model name: %s", m.ModelName())
	}
}

func mockOpenAIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "c1",
			"object":  "chat.completion",
			"created": 1,
			"model":   "llama3.2",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "pong",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
}

func TestBuilder_Chain(t *testing.T) {
	server := mockOpenAIServer()
	defer server.Close()

	m, err := NewBuilder().
		ModelName("custom").
		BaseURL(server.URL).
		APIKey("key").
		Retry(2, time.Millisecond).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != "custom" {
		t.Fatalf("expected custom, got %s", m.ModelName())
	}
}

func TestOllamaChat(t *testing.T) {
	server := mockOpenAIServer()
	defer server.Close()

	m, _ := NewBuilder().BaseURL(server.URL).Build()
	resp, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("ping").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "pong" {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
}

func TestOllamaChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintln(w, "data: [DONE]")
		flusher.Flush()
	}))
	defer server.Close()

	m, _ := NewBuilder().BaseURL(server.URL).Build()
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("ping").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var deltas []string
	for chunk := range ch {
		if chunk != nil && !chunk.Done {
			deltas = append(deltas, chunk.Delta)
		}
	}
	if len(deltas) != 1 || deltas[0] != "pong" {
		t.Fatalf("unexpected: %v", deltas)
	}
}

func TestOllamaChat_WithOptions(t *testing.T) {
	server := mockOpenAIServer()
	defer server.Close()

	m, _ := NewBuilder().BaseURL(server.URL).Build()
	_, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("ping").Build(),
	}, model.WithTemperature(0.5))
	if err != nil {
		t.Fatal(err)
	}
}
