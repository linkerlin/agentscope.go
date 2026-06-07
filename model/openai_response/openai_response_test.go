package openai_response

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestOpenAIResponseModel_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"id": "resp_123",
			"output": [
				{"type": "message", "content": [{"type": "output_text", "text": "Hello from Response API"}]}
			],
			"usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}
		}`)
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").ModelName("o3").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}

	msg := message.NewMsg().Role(message.RoleUser).TextContent("Hi").Build()
	resp, err := m.Chat(context.Background(), []*message.Msg{msg})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetTextContent(), "Hello from Response API") {
		t.Fatalf("unexpected response: %s", resp.GetTextContent())
	}
}

func TestOpenAIResponseModel_Chat_FunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"id": "resp_456",
			"output": [
				{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"Beijing\"}"}
			]
		}`)
	}))
	defer server.Close()

	m, _ := Builder().APIKey("test-key").ModelName("o3").BaseURL(server.URL).Build()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("weather?").Build()
	resp, err := m.Chat(context.Background(), []*message.Msg{msg})
	if err != nil {
		t.Fatal(err)
	}
	// Should contain a ToolUseBlock
	found := false
	for _, b := range resp.Content {
		if tu, ok := b.(*message.ToolUseBlock); ok {
			found = true
			if tu.Name != "get_weather" {
				t.Fatalf("unexpected tool name: %s", tu.Name)
			}
		}
	}
	if !found {
		t.Fatal("expected ToolUseBlock in response")
	}
}

func TestOpenAIResponseModel_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"type\": \"response.output_text.delta\", \"delta\": \"Hello\"}\n\n")
		fmt.Fprint(w, "data: {\"type\": \"response.output_text.delta\", \"delta\": \" World\"}\n\n")
		fmt.Fprint(w, "data: {\"type\": \"response.completed\", \"response\": {\"usage\": {\"input_tokens\": 3, \"output_tokens\": 2, \"total_tokens\": 5}}}\n\n")
	}))
	defer server.Close()

	m, _ := Builder().APIKey("test-key").ModelName("o3").BaseURL(server.URL).Build()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("Hi").Build()
	ch, err := m.ChatStream(context.Background(), []*message.Msg{msg})
	if err != nil {
		t.Fatal(err)
	}

	var deltas []string
	var done bool
	for chunk := range ch {
		if chunk.Done {
			done = true
			if chunk.Usage == nil {
				t.Fatal("expected usage in final chunk")
			}
			if chunk.Usage.TotalTokens != 5 {
				t.Fatalf("expected 5 total tokens, got %d", chunk.Usage.TotalTokens)
			}
		} else {
			deltas = append(deltas, chunk.Delta)
		}
	}
	if !done {
		t.Fatal("expected done chunk")
	}
	if strings.Join(deltas, "") != "Hello World" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
}

func TestOpenAIResponseModel_Chat_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid key"}`)
	}))
	defer server.Close()

	m, _ := Builder().APIKey("bad-key").ModelName("o3").BaseURL(server.URL).Build()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("Hi").Build()
	_, err := m.Chat(context.Background(), []*message.Msg{msg})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIResponseModel_Builder_MissingAPIKey(t *testing.T) {
	_, err := Builder().Build()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestOpenAIResponseModel_Chat_WithTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"resp_789","output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}]}`)
	}))
	defer server.Close()

	m, _ := Builder().APIKey("test-key").ModelName("o3").BaseURL(server.URL).Build()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("test").Build()
	toolSpec := model.ToolSpec{Name: "test_tool", Description: "A test tool"}
	resp, err := m.Chat(context.Background(), []*message.Msg{msg}, model.WithTools([]model.ToolSpec{toolSpec}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "OK" {
		t.Fatalf("unexpected: %s", resp.GetTextContent())
	}
}
