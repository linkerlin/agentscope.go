package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestChatStream_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"get_current_time"}}`,
			"",
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"tz\":\""}}`,
			"",
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"UTC\"}"}}`,
			"",
			`data: {"type":"content_block_stop","index":0}`,
			"",
			`data: {"type":"message_delta","usage":{"output_tokens":7}}`,
			"",
			"data: [DONE]",
			"",
		}
		for _, line := range lines {
			_, _ = w.Write([]byte(line + "\n"))
		}
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("what time is it").Build(),
	})
	if err != nil {
		t.Fatalf("chat stream failed: %v", err)
	}

	var done *model.StreamChunk
	for chunk := range ch {
		if chunk.Done {
			done = chunk
		}
	}
	if done == nil {
		t.Fatal("expected done chunk")
	}
	if len(done.Content) != 1 {
		t.Fatalf("expected 1 tool_use block in done content, got %d", len(done.Content))
	}
	tu, ok := done.Content[0].(*message.ToolUseBlock)
	if !ok {
		t.Fatalf("expected ToolUseBlock, got %T", done.Content[0])
	}
	if tu.Name != "get_current_time" || tu.ID != "tu_1" {
		t.Fatalf("unexpected tool_use: name=%s id=%s", tu.Name, tu.ID)
	}
	if tu.Input["tz"] != "UTC" {
		t.Fatalf("unexpected input: %+v", tu.Input)
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

// TestChat_MaxTokensOverride 验证 WithMaxTokens 覆盖 Builder 默认值。
func TestChat_MaxTokensOverride(t *testing.T) {
	var maxTokens int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		if n, ok := body["max_tokens"].(float64); ok {
			maxTokens = int(n)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).MaxTokens(4096).Build()
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}, model.WithMaxTokens(123))
	if err != nil {
		t.Fatal(err)
	}
	if maxTokens != 123 {
		t.Fatalf("expected max_tokens=123 override, got %d", maxTokens)
	}
}

// TestChatStream_ErrorEvent 验证流中途 error 事件透传到 done chunk 的 Error。
func TestChatStream_ErrorEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}`,
			``,
			`data: {"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, done := drainStream(ch)
	if done == nil {
		t.Fatal("expected done chunk")
	}
	if done.Error == nil {
		t.Fatal("expected Error on done chunk")
	}
	if !strings.Contains(done.Error.Error(), "overloaded") {
		t.Fatalf("unexpected error: %v", done.Error)
	}
}

// TestChatStream_ErrorEventNoMessage 验证 error 事件缺失 message 字段时使用默认错误文本。
func TestChatStream_ErrorEventNoMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			`data: {"type":"error","error":{"type":"overloaded_error"}}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, done := drainStream(ch)
	if done == nil || done.Error == nil {
		t.Fatal("expected done chunk with Error")
	}
	if !strings.Contains(done.Error.Error(), "anthropic stream error") {
		t.Fatalf("expected default error text, got %v", done.Error)
	}
}

// writeSSE 逐行写入 SSE 并 flush;空串 "" 产生事件分隔空行。
func writeSSE(w http.ResponseWriter, lines []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	for _, line := range lines {
		_, _ = w.Write([]byte(line + "\n"))
	}
	if flusher != nil {
		flusher.Flush()
	}
}

// drainStream 读尽 channel,返回 (text deltas, done chunk)。
func drainStream(ch <-chan *model.StreamChunk) ([]string, *model.StreamChunk) {
	var deltas []string
	var done *model.StreamChunk
	for chunk := range ch {
		if chunk.Done {
			done = chunk
			continue
		}
		if chunk.Delta != "" {
			deltas = append(deltas, chunk.Delta)
		}
	}
	return deltas, done
}

// TestChatStream_MessageStopTermination 验证 Anthropic 标准 message_stop 终止(不发 [DONE])。
func TestChatStream_MessageStopTermination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			`data: {"type":"message_start","usage":{"input_tokens":3,"output_tokens":0}}`,
			``,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
			``,
			`data: {"type":"message_delta","usage":{"output_tokens":2}}`,
			``,
			`data: {"type":"message_stop"}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	deltas, done := drainStream(ch)
	if done == nil {
		t.Fatal("expected done chunk on message_stop")
	}
	if len(deltas) != 1 || deltas[0] != "hi" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
	if done.Usage == nil || done.Usage.PromptTokens != 3 || done.Usage.CompletionTokens != 2 {
		t.Fatalf("unexpected usage: %+v", done.Usage)
	}
	if done.Usage.TotalTokens != 5 {
		t.Fatalf("expected total 5, got %d", done.Usage.TotalTokens)
	}
}

// TestChatStream_MultiToolUseOrder 验证多 tool_use 按 content_block index 排序输出,
// 即便 index 1 的块先于 index 0 到达。
func TestChatStream_MultiToolUseOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			// index 1 先到达
			`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_b","name":"second"}}`,
			``,
			`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"b\":1}"}}`,
			``,
			`data: {"type":"content_block_stop","index":1}`,
			``,
			// index 0 后到达
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_a","name":"first"}}`,
			``,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":1}"}}`,
			``,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`data: {"type":"message_stop"}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("go").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, done := drainStream(ch)
	if done == nil || len(done.Content) != 2 {
		t.Fatalf("expected 2 tool blocks, got %#v", done)
	}
	tu0 := done.Content[0].(*message.ToolUseBlock)
	tu1 := done.Content[1].(*message.ToolUseBlock)
	if tu0.Name != "first" || tu0.ID != "tu_a" || tu0.Input["a"] != float64(1) {
		t.Fatalf("first tool wrong: %+v", tu0)
	}
	if tu1.Name != "second" || tu1.ID != "tu_b" || tu1.Input["b"] != float64(1) {
		t.Fatalf("second tool wrong: %+v", tu1)
	}
}

// TestChatStream_TextAndToolMixed 验证 text delta 与 tool_use 在同一流中并存。
func TestChatStream_TextAndToolMixed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"calling tool"}}`,
			``,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"run"}}`,
			``,
			`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"x\":9}"}}`,
			``,
			`data: {"type":"content_block_stop","index":1}`,
			``,
			`data: {"type":"message_stop"}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("go").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	deltas, done := drainStream(ch)
	if len(deltas) != 1 || deltas[0] != "calling tool" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
	if done == nil || len(done.Content) != 1 {
		t.Fatalf("expected 1 tool block, got %#v", done)
	}
	tu := done.Content[0].(*message.ToolUseBlock)
	if tu.Name != "run" || tu.Input["x"] != float64(9) {
		t.Fatalf("unexpected tool: %+v", tu)
	}
}

// TestChatStream_EmptyToolInput 验证 tool input 为空对象 {} 时 Input 为非 nil 空 map,
// 且 RawInput 保留原始 "{}"。
func TestChatStream_EmptyToolInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"noop"}}`,
			``,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
			``,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`data: {"type":"message_stop"}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("go").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, done := drainStream(ch)
	if done == nil || len(done.Content) != 1 {
		t.Fatalf("expected 1 tool block, got %#v", done)
	}
	tu := done.Content[0].(*message.ToolUseBlock)
	if tu.Input == nil {
		t.Fatal("Input should be non-nil empty map for {}")
	}
	if len(tu.Input) != 0 {
		t.Fatalf("expected empty input map, got %+v", tu.Input)
	}
	if tu.RawInput != "{}" {
		t.Fatalf("expected RawInput {}, got %q", tu.RawInput)
	}
}

// TestChatStream_LargeToolInput 验证多分片 input_json_delta 拼接成完整 JSON,
// 含字符串内的转义引号。
func TestChatStream_LargeToolInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, []string{
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"echo"}}`,
			``,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"msg\":\"hel"}}`,
			``,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"lo \\\"world\\\"\"}"}}`,
			``,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`data: {"type":"message_stop"}`,
			``,
		})
	}))
	defer server.Close()

	m, err := NewBuilder().APIKey("k").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("go").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, done := drainStream(ch)
	if done == nil || len(done.Content) != 1 {
		t.Fatalf("expected 1 tool block, got %#v", done)
	}
	tu := done.Content[0].(*message.ToolUseBlock)
	if msg, _ := tu.Input["msg"].(string); msg != `hello "world"` {
		t.Fatalf("expected escaped quote handling, got RawInput=%q Input=%+v", tu.RawInput, tu.Input)
	}
}

// TestChatStream_Integration 是真实 API 集成测试,默认跳过。
// 设置 ANTHROPIC_API_KEY (可选 ANTHROPIC_MODEL / ANTHROPIC_BASE_URL) 后运行:
//
//	ANTHROPIC_API_KEY=sk-... go test ./model/anthropic/ -run TestChatStream_Integration -count=1 -v
func TestChatStream_Integration(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping Anthropic integration test")
	}
	modelName := os.Getenv("ANTHROPIC_MODEL")
	if modelName == "" {
		modelName = "claude-3-5-sonnet-20241022"
	}
	b := NewBuilder().APIKey(apiKey).ModelName(modelName)
	if url := os.Getenv("ANTHROPIC_BASE_URL"); url != "" {
		b.BaseURL(url)
	}
	m, err := b.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ch, err := m.ChatStream(ctx, []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("Reply with exactly the single word: pong").Build(),
	})
	if err != nil {
		t.Fatalf("chat stream failed: %v", err)
	}
	deltas, done := drainStream(ch)
	if done == nil {
		t.Fatal("expected done chunk")
	}
	if len(deltas) == 0 {
		t.Fatal("expected at least one text delta")
	}
	t.Logf("integration reply: %q", strings.Join(deltas, ""))
	if done.Usage != nil {
		t.Logf("usage: in=%d out=%d total=%d", done.Usage.PromptTokens, done.Usage.CompletionTokens, done.Usage.TotalTokens)
	}
}
