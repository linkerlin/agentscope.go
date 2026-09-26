package openai_response

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// TestOpenAIResponseModel_ChatStream_FunctionCall verifies streaming function
// calls are assembled into ToolUseBlocks instead of leaking their argument
// JSON into the answer text (E5b).
func TestOpenAIResponseModel_ChatStream_FunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":""}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"city\""}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":":\"Beijing\"}"}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"response.function_call_arguments.done","output_index":0,"item_id":"call_1","arguments":"{\"city\":\"Beijing\"}"}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`+"\n\n")
	}))
	defer server.Close()

	m, _ := Builder().APIKey("test-key").ModelName("o3").BaseURL(server.URL).Build()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("weather?").Build()
	ch, err := m.ChatStream(context.Background(), []*message.Msg{msg})
	if err != nil {
		t.Fatal(err)
	}

	var textDeltas []string
	var done *model.StreamChunk
	for chunk := range ch {
		if chunk.Done {
			done = chunk
			continue
		}
		textDeltas = append(textDeltas, chunk.Delta)
	}
	if len(textDeltas) != 0 {
		t.Fatalf("function-call arguments must not leak into text, got %v", textDeltas)
	}
	if done == nil {
		t.Fatal("expected done chunk")
	}
	var found *message.ToolUseBlock
	for _, b := range done.Content {
		if tu, ok := b.(*message.ToolUseBlock); ok {
			found = tu
		}
	}
	if found == nil {
		t.Fatalf("expected ToolUseBlock in done chunk, got %+v", done.Content)
	}
	if found.ID != "call_1" || found.Name != "get_weather" {
		t.Fatalf("unexpected tool call identity: %+v", found)
	}
	if city, _ := found.Input["city"].(string); city != "Beijing" {
		t.Fatalf("unexpected tool input: %v", found.Input)
	}
}

// TestOpenAIResponseModel_Chat_FunctionCall_CallID verifies the ToolUseBlock
// ID is the Responses call_id so the later function_call_output round-trips (E5c).
func TestOpenAIResponseModel_Chat_FunctionCall_CallID(t *testing.T) {
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
	for _, b := range resp.Content {
		if tu, ok := b.(*message.ToolUseBlock); ok {
			if tu.ID != "call_1" {
				t.Fatalf("expected call_id as block ID, got %q", tu.ID)
			}
			return
		}
	}
	t.Fatal("expected ToolUseBlock in response")
}

// TestOpenAIResponseModel_BuildRequestBody_ToolHistory verifies assistant tool
// calls and tool results are sent as native function_call/function_call_output
// items so multi-turn tool use round-trips instead of being flattened to
// text (E5c).
func TestOpenAIResponseModel_BuildRequestBody_ToolHistory(t *testing.T) {
	m, err := Builder().APIKey("test-key").ModelName("o3").Build()
	if err != nil {
		t.Fatal(err)
	}
	history := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("weather?").Build(),
		message.NewMsg().Role(message.RoleAssistant).Content(
			message.NewTextBlock("checking"),
			message.NewToolUseBlock("call_1", "get_weather", map[string]any{"city": "Beijing"}),
		).Build(),
		message.NewMsg().Role(message.RoleTool).Content(
			message.NewToolResultBlock("call_1", []message.ContentBlock{
				message.NewTextBlock("sunny"),
			}, false),
		).Build(),
	}
	body, err := m.buildRequestBody(history, false)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`"type":"function_call"`,
		`"call_id":"call_1"`,
		`"name":"get_weather"`,
		`"type":"function_call_output"`,
		`"output":"sunny"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected request body to contain %s, got: %s", want, text)
		}
	}
}

// TestOpenAIResponseModel_BuildRequestBody_ToolOutputMedia verifies tool
// results carrying images use the native content-part array (input_text /
// input_image) instead of dropping the media (native multimodal outputs).
func TestOpenAIResponseModel_BuildRequestBody_ToolOutputMedia(t *testing.T) {
	m, err := Builder().APIKey("test-key").ModelName("o3").Build()
	if err != nil {
		t.Fatal(err)
	}
	history := []*message.Msg{
		message.NewMsg().Role(message.RoleTool).Content(
			message.NewToolResultBlock("call_1", []message.ContentBlock{
				message.NewTextBlock("see:"),
				message.NewImageBlock("", "aGVsbG8=", "image/png"),
			}, false),
		).Build(),
	}
	body, err := m.buildRequestBody(history, false)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`"type":"input_text"`,
		`"type":"input_image"`,
		`data:image/png;base64,aGVsbG8=`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected request body to contain %s, got: %s", want, text)
		}
	}
}

// TestOpenAIResponseModel_ContextCancelClosesStream verifies a cancelled
// consumer does not leave the producer goroutine blocked forever (E5d).
func TestOpenAIResponseModel_ContextCancelClosesStream(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"x"}`+"\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-block // keep the connection open until the test finishes
	}))
	defer server.Close()
	defer close(block)

	m, err := Builder().APIKey("test-key").ModelName("o3").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.ChatStream(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatalf("chat stream failed: %v", err)
	}
	<-ch // consume the first delta
	cancel()

	closed := make(chan struct{})
	go func() {
		for range ch {
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not close after context cancellation (producer leak)")
	}
}
