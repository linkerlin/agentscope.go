// model/ark/ark_test.go — the 20.3 acceptance suite: doubao-seed requests,
// streaming, tools and structured output verified against an httptest
// contract server; the formatter keeps every content block (reasoning_content
// included).
package ark

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func newTestModel(t *testing.T, handler http.HandlerFunc) (*ArkChatModel, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m, err := Builder("ark-key").BaseURL(srv.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	return m, srv
}

// TestChat_RequestContract locks the request wire: endpoint, bearer, model
// name, the Ark thinking switch, tool shape — and the response parsing
// keeping EVERY content block (reasoning_content → ThinkingBlock first).
func TestChat_RequestContract(t *testing.T) {
	var gotAuth, gotPath string
	var body map[string]any
	m, _ := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"index": 0,
				"message": {
					"content": "北京晴",
					"reasoning_content": "查一下天气",
					"tool_calls": [{
						"id": "call_1", "type": "function",
						"function": {"name": "weather", "arguments": "{\"city\":\"北京\"}"}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	})

	msg, err := m.Chat(context.Background(),
		[]*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("北京天气").Build()},
		model.WithTools([]model.ToolSpec{{Name: "weather", Description: "query weather", Parameters: map[string]any{"type": "object"}}}),
	)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path: %s", gotPath)
	}
	if gotAuth != "Bearer ark-key" {
		t.Fatalf("auth: %s", gotAuth)
	}
	if body["model"] != ModelDoubaoSeed {
		t.Fatalf("model: %v", body["model"])
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages: %v", body["messages"])
	}
	if first, _ := msgs[0].(map[string]any); first["content"] != "北京天气" {
		t.Fatalf("first message: %v", msgs[0])
	}
	// Thinking defaults OFF at the builder level: the switch is explicit.
	if _, has := body["thinking"]; has {
		t.Fatalf("thinking must not be sent unless explicitly enabled: %v", body["thinking"])
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools: %v", body["tools"])
	}

	// No content block dropped: thinking, text and tool use all survive.
	if got := msg.GetThinkingContent(); got != "查一下天气" {
		t.Fatalf("thinking content dropped: %q", got)
	}
	if got := msg.GetTextContent(); got != "北京晴" {
		t.Fatalf("text content dropped: %q", got)
	}
	tu := msg.GetToolUseCalls()
	if len(tu) != 1 || tu[0].Name != "weather" || tu[0].ID != "call_1" || tu[0].Input["city"] != "北京" {
		t.Fatalf("tool use dropped: %+v", tu)
	}
}

// TestChat_ThinkingSwitch: the explicit Thinking builder option lands on the
// wire as the Ark `thinking` parameter.
func TestChat_ThinkingSwitch(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	m, err := Builder("k").BaseURL(srv.URL).Thinking(true).Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Chat(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}); err != nil {
		t.Fatal(err)
	}
	th, ok := body["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" {
		t.Fatalf("thinking switch: %v", body["thinking"])
	}
}

// TestChat_ThinkingUnifiedOption: the unified WithThinking option (20.4)
// overrides the builder default on BOTH paths — per-call false beats
// builder-enabled, unset falls back, and the streaming body carries the
// same serialization.
func TestChat_ThinkingUnifiedOption(t *testing.T) {
	var bodies []map[string]any
	m, _ := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	})
	enabled, _ := Builder("k").Thinking(true).BaseURL(m.baseURL).Build()
	_ = enabled
	ctx := context.Background()
	msgs := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()}

	// Builder ON, per-call OFF: per-call wins.
	on, err := Builder("k").BaseURL(m.baseURL).Thinking(true).Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := on.Chat(ctx, msgs, model.WithThinking(false)); err != nil {
		t.Fatal(err)
	}
	off, _ := bodies[len(bodies)-1]["thinking"].(map[string]any)
	if off["type"] != "disabled" {
		t.Fatalf("per-call false must override builder-enabled: %v", bodies[len(bodies)-1]["thinking"])
	}

	// Builder OFF (default), per-call ON: lands enabled.
	if _, err := m.Chat(ctx, msgs, model.WithThinking(true)); err != nil {
		t.Fatal(err)
	}
	th, _ := bodies[len(bodies)-1]["thinking"].(map[string]any)
	if th["type"] != "enabled" {
		t.Fatalf("per-call true: %v", bodies[len(bodies)-1]["thinking"])
	}

	// Neither: absent from the wire.
	if _, err := m.Chat(ctx, msgs); err != nil {
		t.Fatal(err)
	}
	if _, has := bodies[len(bodies)-1]["thinking"]; has {
		t.Fatalf("unset thinking must stay off the wire")
	}

	// Streaming path: same serialization.
	streamBody := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &streamBody)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	sm, err := Builder("k").BaseURL(srv.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := sm.ChatStream(ctx, msgs, model.WithThinking(true))
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	sth, _ := streamBody["thinking"].(map[string]any)
	if sth["type"] != "enabled" {
		t.Fatalf("stream thinking wire: %v", streamBody["thinking"])
	}
}

// TestChat_StructuredOutput: json_object and json_schema both reach the wire
// in the OpenAI-compatible response_format shape.
func TestChat_StructuredOutput(t *testing.T) {
	var bodies []map[string]any
	m, _ := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"x\":1}"}}]}`))
	})
	ctx := context.Background()
	msgs := []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("give json").Build()}

	if _, err := m.Chat(ctx, msgs, model.WithResponseFormat(&model.ResponseFormat{Type: "json_object"})); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Chat(ctx, msgs, model.WithResponseFormat(&model.ResponseFormat{
		Type:       "json_schema",
		JSONSchema: map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "integer"}}},
	})); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("bodies: %d", len(bodies))
	}
	rf1, _ := bodies[0]["response_format"].(map[string]any)
	if rf1["type"] != "json_object" {
		t.Fatalf("json_object wire: %v", bodies[0]["response_format"])
	}
	rf2, _ := bodies[1]["response_format"].(map[string]any)
	js, _ := rf2["json_schema"].(map[string]any)
	schema, _ := js["schema"].(map[string]any)
	if rf2["type"] != "json_schema" || schema == nil || schema["type"] != "object" {
		t.Fatalf("json_schema wire: %v", bodies[1]["response_format"])
	}
}

// TestChatStream_DeltaFlow: SSE reasoning deltas stream as IsThinking chunks,
// content as regular deltas, tool calls as content blocks, and the usage
// event closes the stream.
func TestChatStream_DeltaFlow(t *testing.T) {
	m, _ := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		events := []string{
			`{"choices":[{"index":0,"delta":{"reasoning_content":"思考中"}}]}`,
			`{"choices":[{"index":0,"delta":{"content":"答"}}]}`,
			`{"choices":[{"index":0,"delta":{"content":"案"}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_9","type":"function","function":{"name":"f","arguments":"{}"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		}
		for _, ev := range events {
			_, _ = w.Write([]byte("data: " + ev + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if fl != nil {
			fl.Flush()
		}
	})

	ch, err := m.ChatStream(context.Background(),
		[]*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("go").Build()})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var thinking, text string
	var toolBlocks int
	var usage *model.ChatUsage
	for chunk := range ch {
		if chunk.IsThinking {
			thinking += chunk.Delta
			continue
		}
		if chunk.Delta != "" {
			text += chunk.Delta
		}
		toolBlocks += len(chunk.Content)
		if chunk.Done {
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
		}
	}
	if thinking != "思考中" {
		t.Fatalf("thinking stream: %q", thinking)
	}
	if text != "答案" {
		t.Fatalf("text stream: %q", text)
	}
	if toolBlocks != 1 {
		t.Fatalf("tool blocks: %d", toolBlocks)
	}
	if usage == nil || usage.PromptTokens != 3 || usage.TotalTokens != 5 {
		t.Fatalf("usage: %+v", usage)
	}
}

// TestChat_Error: non-200 with the Ark error envelope surfaces code+message.
func TestChat_Error(t *testing.T) {
	m, _ := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"AuthenticationError","message":"invalid api key"}}`))
	})
	_, err := m.Chat(context.Background(), []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("x").Build()})
	if err == nil {
		t.Fatal("must fail")
	}
	if !strings.Contains(err.Error(), "AuthenticationError") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error: %v", err)
	}
}
