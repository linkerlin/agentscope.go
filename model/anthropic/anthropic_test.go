package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestBuilder_RequiresAPIKey(t *testing.T) {
	_, err := NewBuilder().Build()
	if err == nil {
		t.Fatal("expected error without API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChat_Success(t *testing.T) {
	var reqBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &reqBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"content": []map[string]any{{"type": "text", "text": "hello world"}},
			"usage":   map[string]any{"input_tokens": 10, "output_tokens": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).ModelName("claude-test").MaxTokens(100).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("sys").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("user msg").Build(),
	}
	tools := []model.ToolSpec{
		{Name: "calc", Description: "a calculator", Parameters: map[string]any{"type": "object"}},
	}
	tc := &model.ToolChoice{Mode: "auto"}

	resp, err := m.Chat(context.Background(), msgs, model.WithTemperature(0.7), model.WithTools(tools), model.WithToolChoice(tc))
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if resp.GetTextContent() != "hello world" {
		t.Fatalf("unexpected response text: %s", resp.GetTextContent())
	}

	if reqBody["model"] != "claude-test" {
		t.Errorf("expected model claude-test, got %v", reqBody["model"])
	}
	if reqBody["max_tokens"] != float64(100) {
		t.Errorf("expected max_tokens 100, got %v", reqBody["max_tokens"])
	}
	if reqBody["temperature"] != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", reqBody["temperature"])
	}
	if reqBody["system"] != "sys" {
		t.Errorf("expected system sys, got %v", reqBody["system"])
	}
	if _, ok := reqBody["messages"]; !ok {
		t.Error("expected messages in request body")
	}
	if _, ok := reqBody["tools"]; !ok {
		t.Error("expected tools in request body")
	}
	if _, ok := reqBody["tool_choice"]; !ok {
		t.Error("expected tool_choice in request body")
	}
}

func TestChat_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	_, err = m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 in error, got: %v", err)
	}
}

func TestChatStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("expected Accept: text/event-stream, got %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}
		lines := []string{
			`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
			"",
			`data: {"type":"message_delta","usage":{"output_tokens":10}}`,
			"",
			"data: [DONE]",
			"",
		}
		for _, line := range lines {
			_, _ = w.Write([]byte(line + "\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatalf("chat stream failed: %v", err)
	}

	var deltas []string
	var done *model.StreamChunk
	for chunk := range ch {
		if chunk.Done {
			done = chunk
			continue
		}
		deltas = append(deltas, chunk.Delta)
	}

	if len(deltas) != 1 || deltas[0] != "hello" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
	if done == nil {
		t.Fatal("expected done chunk")
	}
	if done.Usage == nil || done.Usage.CompletionTokens != 10 {
		t.Fatalf("unexpected usage: %+v", done.Usage)
	}
}

func TestChatStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	_, err = m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 in error, got: %v", err)
	}
}

func TestChat_Retry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"content": []map[string]any{{"type": "text", "text": "retry ok"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).Retry(3, 10*time.Millisecond).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	resp, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatalf("chat failed after retry: %v", err)
	}
	if resp.GetTextContent() != "retry ok" {
		t.Fatalf("unexpected text: %s", resp.GetTextContent())
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestChatStream_Retry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"stream retry ok\"}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).Retry(3, 10*time.Millisecond).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatalf("chat stream failed after retry: %v", err)
	}

	var deltas []string
	var done bool
	for chunk := range ch {
		if chunk.Done {
			done = true
			continue
		}
		deltas = append(deltas, chunk.Delta)
	}

	if !done {
		t.Fatal("expected done flag")
	}
	if len(deltas) != 1 || deltas[0] != "stream retry ok" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}
