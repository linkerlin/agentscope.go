package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestBuildRequestBody(t *testing.T) {
	m, err := NewBuilder().APIKey("test-key").ModelName("gemini-pro").Build()
	if err != nil {
		t.Fatal(err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("system prompt").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("hello").Build(),
	}
	body, err := m.buildRequestBody(msgs, false, model.WithTemperature(0.5), model.WithMaxTokens(100))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `"contents"`) {
		t.Errorf("expected contents in body: %s", s)
	}
	if !strings.Contains(s, `"systemInstruction"`) {
		t.Errorf("expected systemInstruction in body: %s", s)
	}
	if !strings.Contains(s, `"temperature"`) {
		t.Errorf("expected temperature in body: %s", s)
	}
	if !strings.Contains(s, `"maxOutputTokens"`) {
		t.Errorf("expected maxOutputTokens in body: %s", s)
	}
}

func TestBuildRequestBodyWithTools(t *testing.T) {
	m, err := NewBuilder().APIKey("test-key").Build()
	if err != nil {
		t.Fatal(err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}
	tools := []model.ToolSpec{
		{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}},
	}
	body, err := m.buildRequestBody(msgs, false, model.WithTools(tools), model.WithToolChoice(&model.ToolChoice{Mode: "any", Function: "echo"}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `"functionDeclarations"`) {
		t.Errorf("expected functionDeclarations in body: %s", s)
	}
	if !strings.Contains(s, `"functionCallingConfig"`) {
		t.Errorf("expected functionCallingConfig in body: %s", s)
	}
	if !strings.Contains(s, `"allowedFunctionNames"`) {
		t.Errorf("expected allowedFunctionNames in body: %s", s)
	}
}

func TestParseResponse(t *testing.T) {
	m, _ := NewBuilder().APIKey("test-key").Build()
	resp := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role":  "model",
					"parts": []any{map[string]any{"text": "world"}},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     1.0,
			"candidatesTokenCount": 2.0,
			"totalTokenCount":      3.0,
		},
	}
	msg, err := m.fmt.ParseResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetTextContent() != "world" {
		t.Errorf("expected 'world', got %q", msg.GetTextContent())
	}
	usage, _ := msg.Metadata["usage"].(model.ChatUsage)
	if usage.TotalTokens != 3 {
		t.Errorf("expected total tokens 3, got %d", usage.TotalTokens)
	}
}

func TestBuilder_ChainAndBuildDefaults(t *testing.T) {
	m, err := NewBuilder().
		APIKey("k").
		BaseURL("http://localhost:8080").
		ModelName("gemini-test").
		Retry(3, time.Second).
		Formatter(formatter.NewGeminiFormatter()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelName() != "gemini-test" {
		t.Fatalf("expected gemini-test, got %s", m.ModelName())
	}
}

func TestChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []any{
				map[string]any{
					"content": map[string]any{
						"role":  "model",
						"parts": []any{map[string]any{"text": "hello"}},
					},
				},
			},
		})
	}))
	defer server.Close()

	m, _ := NewBuilder().APIKey("k").BaseURL(server.URL).ModelName("gemini-pro").Build()
	resp, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "hello" {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
}

func TestChat_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	m, _ := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	_, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":streamGenerateContent") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, text := range []string{"hello", " world"} {
			payload, _ := json.Marshal(map[string]any{
				"candidates": []any{
					map[string]any{
						"content": map[string]any{
							"parts": []any{map[string]any{"text": text}},
						},
					},
				},
			})
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
		fmt.Fprintln(w, "data: [DONE]")
		flusher.Flush()
	}))
	defer server.Close()

	m, _ := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
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
	if len(deltas) != 2 || deltas[0] != "hello" || deltas[1] != " world" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
}

func TestChatStream_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	m, _ := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	_, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChat_WithRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "transient", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": "ok"}}}}},
		})
	}))
	defer server.Close()

	m, _ := NewBuilder().APIKey("k").BaseURL(server.URL).Retry(3, 10*time.Millisecond).Build()
	resp, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestIntAny(t *testing.T) {
	if intAny(float64(5)) != 5 {
		t.Fatal("float64")
	}
	if intAny(int(5)) != 5 {
		t.Fatal("int")
	}
	if intAny(int64(5)) != 5 {
		t.Fatal("int64")
	}
	if intAny("x") != 0 {
		t.Fatal("string")
	}
}
