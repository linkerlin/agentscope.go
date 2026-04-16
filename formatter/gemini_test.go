package formatter

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestGemini_NewGeminiFormatter(t *testing.T) {
	f := NewGeminiFormatter()
	if f == nil {
		t.Fatal("expected non-nil GeminiFormatter")
	}
}

func TestGemini_FormatContents_SystemPromptExtraction(t *testing.T) {
	f := NewGeminiFormatter()
	sysMsg := message.NewMsg().Role(message.RoleSystem).TextContent("be helpful").Build()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()

	contents, system := f.FormatContents([]*message.Msg{sysMsg, userMsg})
	if system != "be helpful" {
		t.Fatalf("expected system prompt 'be helpful', got %q", system)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	if contents[0]["role"] != "user" {
		t.Fatalf("expected role user, got %v", contents[0]["role"])
	}
}

func TestGemini_FormatContents_UserAndModelRoles(t *testing.T) {
	f := NewGeminiFormatter()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()
	modelMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("hey").Build()

	contents, _ := f.FormatContents([]*message.Msg{userMsg, modelMsg})
	if len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(contents))
	}
	if contents[0]["role"] != "user" {
		t.Fatalf("expected user role, got %v", contents[0]["role"])
	}
	if contents[1]["role"] != "model" {
		t.Fatalf("expected model role, got %v", contents[1]["role"])
	}
}

func TestGemini_FormatContents_TextAndThinkingParts(t *testing.T) {
	f := NewGeminiFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewTextBlock("hello")).
		Content(message.NewThinkingBlock("think deep", "sig")).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0]["text"] != "hello" {
		t.Fatalf("expected text 'hello', got %v", parts[0]["text"])
	}
	if parts[1]["text"] != "think deep" {
		t.Fatalf("expected text 'think deep', got %v", parts[1]["text"])
	}
}

func TestGemini_FormatContents_ImageBlockBase64(t *testing.T) {
	f := NewGeminiFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewImageBlock("", "b64data", "image/jpeg")).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	inline, ok := parts[0]["inline_data"].(map[string]any)
	if !ok {
		t.Fatalf("expected inline_data, got %v", parts[0])
	}
	if inline["mime_type"] != "image/jpeg" || inline["data"] != "b64data" {
		t.Fatalf("unexpected inline_data: %v", inline)
	}
}

func TestGemini_FormatContents_ImageBlockURL(t *testing.T) {
	f := NewGeminiFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewImageBlock("http://example.com/img.png", "", "")).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	if parts[0]["text"] != "[Image: http://example.com/img.png]" {
		t.Fatalf("unexpected part: %v", parts[0])
	}
}

func TestGemini_FormatContents_AudioAndVideoBlocks(t *testing.T) {
	f := NewGeminiFormatter()
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewAudioBlock("http://a.mp3", "", "")).
		Content(message.NewVideoBlock("http://v.mp4")).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0]["text"] != "[Audio: http://a.mp3]" {
		t.Fatalf("unexpected audio part: %v", parts[0])
	}
	if parts[1]["text"] != "[Video: http://v.mp4]" {
		t.Fatalf("unexpected video part: %v", parts[1])
	}
}

func TestGemini_FormatContents_DataBlockImage(t *testing.T) {
	f := NewGeminiFormatter()
	src := &message.Source{Type: message.SourceTypeBase64, MediaType: "image/png", Data: "d"}
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewDataBlock(message.TypeImage, src)).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	inline, ok := parts[0]["inline_data"].(map[string]any)
	if !ok {
		t.Fatalf("expected inline_data, got %v", parts[0])
	}
	if inline["mime_type"] != "image/png" || inline["data"] != "d" {
		t.Fatalf("unexpected inline_data: %v", inline)
	}
}

func TestGemini_FormatContents_DataBlockNonImage(t *testing.T) {
	f := NewGeminiFormatter()
	src := &message.Source{Type: message.SourceTypeURL, URL: "http://x.com/audio"}
	msg := message.NewMsg().Role(message.RoleUser).
		Content(message.NewDataBlock(message.TypeAudio, src)).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	if parts[0]["text"] != "[audio: http://x.com/audio]" {
		t.Fatalf("unexpected part: %v", parts[0])
	}
}

func TestGemini_FormatContents_EmptyPartsIgnored(t *testing.T) {
	f := NewGeminiFormatter()
	// System message produces empty parts and should be skipped entirely.
	// A message with no recognized blocks produces no parts and should be omitted.
	emptyMsg := message.NewMsg().Role(message.RoleUser).Build()
	userMsg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	contents, _ := f.FormatContents([]*message.Msg{emptyMsg, userMsg})
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
}

func TestGemini_FormatContents_ToolUseAndToolResult(t *testing.T) {
	f := NewGeminiFormatter()
	toolUse := message.NewToolUseBlock("1", "calc", map[string]any{"a": 1})
	toolResult := message.NewToolResultBlock("1", []message.ContentBlock{
		message.NewTextBlock("result is 2"),
	}, false)
	msg := message.NewMsg().Role(message.RoleUser).
		Content(toolUse).
		Content(toolResult).
		Build()

	contents, _ := f.FormatContents([]*message.Msg{msg})
	parts := contents[0]["parts"].([]map[string]any)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0]["text"] != "ToolUse: calc(map[a:1])" {
		t.Fatalf("unexpected tool use part: %v", parts[0])
	}
	if parts[1]["text"] != "result is 2" {
		t.Fatalf("unexpected tool result part: %v", parts[1])
	}
}

func TestGemini_FormatTools(t *testing.T) {
	f := NewGeminiFormatter()
	specs := []model.ToolSpec{
		{
			Name:        "weather",
			Description: "get weather",
			Parameters: map[string]any{
				"type": "object",
			},
		},
	}
	decls := f.FormatTools(specs)
	if len(decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(decls))
	}
	if decls[0]["name"] != "weather" {
		t.Fatalf("unexpected name: %v", decls[0]["name"])
	}
	if decls[0]["description"] != "get weather" {
		t.Fatalf("unexpected description: %v", decls[0]["description"])
	}
	params, ok := decls[0]["parameters"].(map[string]any)
	if !ok || params["type"] != "object" {
		t.Fatalf("unexpected parameters: %v", decls[0]["parameters"])
	}
}

func TestGemini_ParseResponse_TextOnly(t *testing.T) {
	f := NewGeminiFormatter()
	body := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role": "model",
					"parts": []any{
						map[string]any{"text": "hello world"},
					},
				},
			},
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Role != message.RoleAssistant {
		t.Fatalf("expected assistant role, got %v", msg.Role)
	}
	if msg.GetTextContent() != "hello world" {
		t.Fatalf("expected 'hello world', got %q", msg.GetTextContent())
	}
}

func TestGemini_ParseResponse_FunctionCall(t *testing.T) {
	f := NewGeminiFormatter()
	body := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role": "model",
					"parts": []any{
						map[string]any{
							"function_call": map[string]any{
								"name": "calc",
								"args": map[string]any{"x": 10},
							},
						},
					},
				},
			},
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := msg.GetToolUseCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool use call, got %d", len(calls))
	}
	if calls[0].Name != "calc" {
		t.Fatalf("expected tool name 'calc', got %q", calls[0].Name)
	}
	if calls[0].Input["x"] != 10 {
		t.Fatalf("unexpected args: %v", calls[0].Input)
	}
}

func TestGemini_ParseResponse_UsageMetadata(t *testing.T) {
	f := NewGeminiFormatter()
	body := map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"role":  "model",
					"parts": []any{map[string]any{"text": "ok"}},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     5,
			"candidatesTokenCount": 3,
			"totalTokenCount":      8,
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usage, ok := msg.Metadata["usage"].(model.ChatUsage)
	if !ok {
		t.Fatalf("expected usage metadata, got %v", msg.Metadata["usage"])
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 3 || usage.TotalTokens != 8 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestGemini_ParseResponse_NoCandidatesError(t *testing.T) {
	f := NewGeminiFormatter()
	cases := []map[string]any{
		{},
		{"candidates": []any{}},
		{"candidates": "bad"},
	}
	for i, body := range cases {
		_, err := f.ParseResponse(body)
		if err == nil {
			t.Fatalf("case %d: expected error for body %v", i, body)
		}
	}
}
