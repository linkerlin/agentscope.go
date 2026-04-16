package formatter

import (
	"encoding/json"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestNewAnthropicFormatter(t *testing.T) {
	f := NewAnthropicFormatter()
	if f == nil {
		t.Fatal("expected non-nil formatter")
	}
}

func TestAnthropicFormatter_FormatMessages_SystemPrompt(t *testing.T) {
	f := NewAnthropicFormatter()
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("sys").Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	}
	out, sys := f.FormatMessages(msgs)
	if sys != "sys" {
		t.Fatalf("expected system prompt 'sys', got %q", sys)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if out[0].Role != "user" {
		t.Fatalf("expected role user, got %s", out[0].Role)
	}
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0]["type"] != "text" || blocks[0]["text"] != "hi" {
		t.Fatalf("unexpected blocks: %+v", blocks)
	}
}

func TestAnthropicFormatter_FormatMessages_Roles(t *testing.T) {
	f := NewAnthropicFormatter()
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("u").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("a").Build(),
		message.NewMsg().Role(message.RoleTool).TextContent("t").Build(),
	}
	out, _ := f.FormatMessages(msgs)
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
	expected := []string{"user", "assistant", "user"}
	for i, exp := range expected {
		if out[i].Role != exp {
			t.Fatalf("expected role %s at %d, got %s", exp, i, out[i].Role)
		}
	}
}

func TestAnthropicFormatter_FormatMessages_SkipsEmptyContent(t *testing.T) {
	f := NewAnthropicFormatter()
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).Build(),
		message.NewMsg().Role(message.RoleUser).TextContent("x").Build(),
	}
	out, _ := f.FormatMessages(msgs)
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
}

func TestAnthropicFormatter_FormatMessages_ImageBlockURL(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewImageBlock("http://img", "", ""),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	src, ok := blocks[0]["source"].(map[string]any)
	if !ok || src["type"] != "url" || src["url"] != "http://img" {
		t.Fatalf("unexpected image block: %+v", blocks[0])
	}
}

func TestAnthropicFormatter_FormatMessages_ImageBlockBase64(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewImageBlock("", "abc123", "image/jpeg"),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	src, ok := blocks[0]["source"].(map[string]any)
	if !ok || src["type"] != "base64" || src["media_type"] != "image/jpeg" || src["data"] != "abc123" {
		t.Fatalf("unexpected image block: %+v", blocks[0])
	}
}

func TestAnthropicFormatter_FormatMessages_DataBlockImage(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewDataBlock(message.TypeImage, &message.Source{
			Type:      message.SourceTypeBase64,
			MediaType: "image/png",
			Data:      "data",
		}),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	if blocks[0]["type"] != "image" {
		t.Fatalf("expected type image, got %+v", blocks[0])
	}
}

func TestAnthropicFormatter_FormatMessages_DataBlockNilSourceSkipped(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewDataBlock(message.TypeImage, nil),
		message.NewTextBlock("hello"),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0]["text"] != "hello" {
		t.Fatalf("unexpected blocks: %+v", blocks)
	}
}

func TestAnthropicFormatter_FormatMessages_AudioBlockFallback(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewAudioBlock("http://audio", "", ""),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	if blocks[0]["type"] != "text" || blocks[0]["text"] != "[Audio: http://audio]" {
		t.Fatalf("unexpected audio block: %+v", blocks[0])
	}
}

func TestAnthropicFormatter_FormatMessages_VideoBlockFallback(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleUser).Content(
		message.NewVideoBlock("http://video"),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	if blocks[0]["type"] != "text" || blocks[0]["text"] != "[Video: http://video]" {
		t.Fatalf("unexpected video block: %+v", blocks[0])
	}
}

func TestAnthropicFormatter_FormatMessages_ToolResultBlock(t *testing.T) {
	f := NewAnthropicFormatter()
	msg := message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("tu_1", []message.ContentBlock{
			message.NewTextBlock("res"),
		}, false),
	).Build()
	out, _ := f.FormatMessages([]*message.Msg{msg})
	var blocks []map[string]any
	if err := json.Unmarshal(out[0].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	if blocks[0]["type"] != "tool_result" || blocks[0]["tool_use_id"] != "tu_1" || blocks[0]["content"] != "res" {
		t.Fatalf("unexpected tool_result block: %+v", blocks[0])
	}
}

func TestAnthropicFormatter_FormatTools(t *testing.T) {
	f := NewAnthropicFormatter()
	specs := []model.ToolSpec{
		{
			Name:        "calc",
			Description: "calculator",
			Parameters:  map[string]any{"type": "object"},
		},
	}
	out := f.FormatTools(specs)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out))
	}
	if out[0]["name"] != "calc" || out[0]["description"] != "calculator" || out[0]["input_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("unexpected tool: %+v", out[0])
	}
}

func TestAnthropicFormatter_FormatToolChoice(t *testing.T) {
	f := NewAnthropicFormatter()

	tests := []struct {
		name     string
		tc       *model.ToolChoice
		expected map[string]any
	}{
		{"nil", nil, map[string]any{"type": "auto"}},
		{"function", &model.ToolChoice{Function: "foo"}, map[string]any{"type": "tool", "name": "foo"}},
		{"none", &model.ToolChoice{Mode: "none"}, map[string]any{"type": "none"}},
		{"required", &model.ToolChoice{Mode: "required"}, map[string]any{"type": "any"}},
		{"any", &model.ToolChoice{Mode: "any"}, map[string]any{"type": "any"}},
		{"default", &model.ToolChoice{Mode: "auto"}, map[string]any{"type": "auto"}},
		{"unknown", &model.ToolChoice{Mode: "other"}, map[string]any{"type": "auto"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := f.FormatToolChoice(tt.tc)
			if out["type"] != tt.expected["type"] {
				t.Fatalf("expected type %v, got %v", tt.expected["type"], out["type"])
			}
			if name, ok := tt.expected["name"]; ok && out["name"] != name {
				t.Fatalf("expected name %v, got %v", name, out["name"])
			}
		})
	}
}

func TestAnthropicFormatter_ParseResponse_Text(t *testing.T) {
	f := NewAnthropicFormatter()
	body := map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": "hello"},
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.GetTextContent() != "hello" {
		t.Fatalf("expected hello, got %s", msg.GetTextContent())
	}
	if msg.Role != message.RoleAssistant {
		t.Fatalf("expected assistant role, got %s", msg.Role)
	}
}

func TestAnthropicFormatter_ParseResponse_Thinking(t *testing.T) {
	f := NewAnthropicFormatter()
	body := map[string]any{
		"content": []any{
			map[string]any{"type": "thinking", "thinking": "think", "signature": "sig"},
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	blocks := msg.Content
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	th, ok := blocks[0].(*message.ThinkingBlock)
	if !ok || th.Thinking != "think" || th.Signature != "sig" {
		t.Fatalf("unexpected thinking block: %+v", blocks[0])
	}
}

func TestAnthropicFormatter_ParseResponse_ToolUse(t *testing.T) {
	f := NewAnthropicFormatter()
	body := map[string]any{
		"content": []any{
			map[string]any{"type": "tool_use", "id": "tu1", "name": "calc", "input": map[string]any{"x": 1}},
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	calls := msg.GetToolUseCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool use, got %d", len(calls))
	}
	if calls[0].ID != "tu1" || calls[0].Name != "calc" || calls[0].Input["x"] != 1 {
		t.Fatalf("unexpected tool use: %+v", calls[0])
	}
}

func TestAnthropicFormatter_ParseResponse_Usage(t *testing.T) {
	f := NewAnthropicFormatter()
	body := map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": "ok"},
		},
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
	msg, err := f.ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	usage, ok := msg.Metadata["usage"].(model.ChatUsage)
	if !ok {
		t.Fatalf("expected usage metadata, got %+v", msg.Metadata["usage"])
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 5 || usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestAnthropicFormatter_ParseResponse_MissingContentError(t *testing.T) {
	f := NewAnthropicFormatter()
	body := map[string]any{
		"foo": "bar",
	}
	_, err := f.ParseResponse(body)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}
