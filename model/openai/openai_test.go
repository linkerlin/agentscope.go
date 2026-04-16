package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/formatter"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestMsgToOpenAI_TextOnly(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if out[0].Role != "user" || out[0].Content != "hello" {
		t.Fatalf("unexpected message: %+v", out[0])
	}
}

func TestMsgToOpenAI_ImageBlock_MultiContent(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		TextContent("describe").
		Content(message.NewImageBlock("http://example.com/img.png", "", "image/png")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent for image message")
	}
	foundText := false
	foundImage := false
	for _, part := range out[0].MultiContent {
		switch part.Type {
		case goopenai.ChatMessagePartTypeText:
			foundText = true
			if part.Text != "describe" {
				t.Fatalf("expected text 'describe', got %s", part.Text)
			}
		case goopenai.ChatMessagePartTypeImageURL:
			foundImage = true
			if part.ImageURL == nil || part.ImageURL.URL != "http://example.com/img.png" {
				t.Fatalf("unexpected image url: %+v", part.ImageURL)
			}
		}
	}
	if !foundText || !foundImage {
		t.Fatalf("expected both text and image parts, got text=%v image=%v", foundText, foundImage)
	}
}

func TestMsgToOpenAI_ImageBlock_Base64(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewImageBlock("", "iVBORw0KGgo=", "image/png")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent")
	}
	part := out[0].MultiContent[0]
	if part.Type != goopenai.ChatMessagePartTypeImageURL || part.ImageURL == nil {
		t.Fatal("expected image_url part")
	}
	if part.ImageURL.URL != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("unexpected data url: %s", part.ImageURL.URL)
	}
}

func TestMsgToOpenAI_VideoBlock_FallbackToText(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewVideoBlock("http://example.com/vid.mp4")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent")
	}
	part := out[0].MultiContent[0]
	if part.Type != goopenai.ChatMessagePartTypeText {
		t.Fatalf("expected text fallback, got %v", part.Type)
	}
	if part.Text != "[Video: http://example.com/vid.mp4]" {
		t.Fatalf("unexpected fallback text: %s", part.Text)
	}
}

func TestMsgToOpenAI_AudioBlock_FallbackToText(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewAudioBlock("http://example.com/audio.mp3", "", "audio/mp3")).
		Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 1 || len(out[0].MultiContent) == 0 {
		t.Fatal("expected MultiContent")
	}
	part := out[0].MultiContent[0]
	if part.Type != goopenai.ChatMessagePartTypeText {
		t.Fatalf("expected text fallback, got %v", part.Type)
	}
	if part.Text != "[Audio: http://example.com/audio.mp3]" {
		t.Fatalf("unexpected fallback text: %s", part.Text)
	}
}

func TestMsgToOpenAI_ToolResults(t *testing.T) {
	f := formatter.NewOpenAIFormatter()
	msg := message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock("ok")}, false),
		message.NewToolResultBlock("call_2", []message.ContentBlock{message.NewTextBlock("done")}, false),
	).Build()
	out := f.FormatMessagesTyped([]*message.Msg{msg})
	if len(out) != 2 {
		t.Fatalf("expected 2 tool messages, got %d", len(out))
	}
	if out[0].Role != "tool" || out[0].ToolCallID != "call_1" || out[0].Content != "ok" {
		t.Fatalf("unexpected first tool msg: %+v", out[0])
	}
	if out[1].Role != "tool" || out[1].ToolCallID != "call_2" || out[1].Content != "done" {
		t.Fatalf("unexpected second tool msg: %+v", out[1])
	}
}

func TestBuilder_RequiresAPIKey(t *testing.T) {
	_, err := Builder().Build()
	if err == nil || err.Error() != "openai: API key is required" {
		t.Fatalf("expected API key required error, got %v", err)
	}
}

func TestChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if reqBody["model"] != "gpt-4o" {
			http.Error(w, "bad model", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-1",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4o",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").ModelName("gpt-4o").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("system").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}
	tools := []model.ToolSpec{{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}}}
	resp, err := m.Chat(context.Background(), msgs,
		model.WithTemperature(0.5),
		model.WithMaxTokens(100),
		model.WithTools(tools),
		model.WithToolChoice(&model.ToolChoice{Mode: "auto"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "hello" {
		t.Fatalf("expected 'hello', got %q", resp.GetTextContent())
	}
	usage, _ := resp.Metadata["usage"].(model.ChatUsage)
	if usage.TotalTokens != 2 {
		t.Fatalf("expected usage total_tokens 2, got %d", usage.TotalTokens)
	}
}

func TestChat_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate_limit"}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		events := []string{
			`data: {"id":"1","object":"chat.completion.chunk","choices":[{"delta":{"content":"he"}}]}` + "\n\n",
			`data: {"id":"1","object":"chat.completion.chunk","choices":[{"delta":{"content":"llo"}}]}` + "\n\n",
			`data: [DONE]` + "\n\n",
		}
		for _, ev := range events {
			_, _ = w.Write([]byte(ev))
			flusher.Flush()
		}
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var deltas []string
	for chunk := range ch {
		if chunk.Done {
			break
		}
		deltas = append(deltas, chunk.Delta)
	}
	got := strings.Join(deltas, "")
	if got != "hello" {
		t.Fatalf("expected stream 'hello', got %q", got)
	}
}

func TestChatStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").BaseURL(server.URL).Build()
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChat_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, `{"error":"overload"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-1",
			"object": "chat.completion",
			"model":  "gpt-4o",
			"choices": []any{
				map[string]any{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "ok"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").BaseURL(server.URL).Retry(3, 10*time.Millisecond).Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := m.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTextContent() != "ok" {
		t.Fatalf("expected 'ok', got %q", resp.GetTextContent())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestChatStream_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, `{"error":"overload"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}")
		_, _ = fmt.Fprintln(w, "data: [DONE]")
	}))
	defer server.Close()

	m, err := Builder().APIKey("test-key").BaseURL(server.URL).Retry(3, 10*time.Millisecond).Build()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := m.ChatStream(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var deltas []string
	for chunk := range ch {
		if chunk.Done {
			break
		}
		deltas = append(deltas, chunk.Delta)
	}
	if strings.Join(deltas, "") != "ok" {
		t.Fatalf("expected 'ok', got %v", deltas)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
