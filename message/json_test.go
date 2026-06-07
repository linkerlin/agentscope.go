package message

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMsgJSON_DataBlock(t *testing.T) {
	msg := NewMsg().Role(RoleUser).Content(
		NewDataBlock(TypeImage, &Source{Type: SourceTypeURL, URL: "http://x.com/a.png"}),
		NewDataBlock(TypeAudio, &Source{Type: SourceTypeBase64, MediaType: "audio/mp3", Data: "abc"}),
		NewDataBlock(TypeVideo, &Source{Type: SourceTypeURL, URL: "http://x.com/v.mp4"}),
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(parsed.Content) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(parsed.Content))
	}

	// DataBlock serializes into the Python-compatible source-nested format.
	// On deserialization, it is normalized back to the specific block type
	// (ImageBlock/AudioBlock/VideoBlock) for compatibility.
	img := parsed.Content[0].(*ImageBlock)
	if img.URL != "http://x.com/a.png" {
		t.Fatalf("unexpected image block: %+v", img)
	}
	aud := parsed.Content[1].(*AudioBlock)
	if aud.Base64 != "abc" || aud.MimeType != "audio/mp3" {
		t.Fatalf("unexpected audio block: %+v", aud)
	}
	vid := parsed.Content[2].(*VideoBlock)
	if vid.URL != "http://x.com/v.mp4" {
		t.Fatalf("unexpected video block: %+v", vid)
	}
}

func TestMsgJSON_ImageBlock_SourceNested(t *testing.T) {
	msg := NewMsg().Role(RoleUser).Content(
		NewImageBlock("", "iVBORw0KGgo=", "image/png"),
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw failed: %v", err)
	}
	content := raw["content"].([]any)
	block := content[0].(map[string]any)
	if block["type"] != "data" {
		t.Fatalf("expected type data, got %v", block["type"])
	}
	src, ok := block["source"].(map[string]any)
	if !ok {
		t.Fatal("expected nested source object")
	}
	if src["type"] != "base64" || src["data"] != "iVBORw0KGgo=" || src["media_type"] != "image/png" {
		t.Fatalf("unexpected source: %+v", src)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	img := parsed.Content[0].(*ImageBlock)
	if img.Base64 != "iVBORw0KGgo=" || img.MimeType != "image/png" {
		t.Fatalf("unexpected image block: %+v", img)
	}
}

func TestMsgJSON_AudioBlock_SourceNested(t *testing.T) {
	msg := NewMsg().Role(RoleUser).Content(
		NewAudioBlock("http://x.com/a.mp3", "", ""),
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	aud := parsed.Content[0].(*AudioBlock)
	if aud.URL != "http://x.com/a.mp3" {
		t.Fatalf("unexpected audio block: %+v", aud)
	}
}

func TestMsgJSON_BackwardCompatFlatFormat(t *testing.T) {
	oldJSON := `{
		"id":"1","role":"user","created_at":"2024-01-01T00:00:00Z",
		"content":[
			{"type":"image","url":"http://old.com/img.png","mime_type":"image/png"},
			{"type":"audio","base64":"abc","mime_type":"audio/wav"},
			{"type":"video","url":"http://old.com/vid.mp4"}
		]
	}`
	var parsed Msg
	if err := json.Unmarshal([]byte(oldJSON), &parsed); err != nil {
		t.Fatalf("unmarshal old format failed: %v", err)
	}
	if len(parsed.Content) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(parsed.Content))
	}
	img := parsed.Content[0].(*ImageBlock)
	if img.URL != "http://old.com/img.png" {
		t.Fatalf("unexpected image: %+v", img)
	}
	aud := parsed.Content[1].(*AudioBlock)
	if aud.Base64 != "abc" {
		t.Fatalf("unexpected audio: %+v", aud)
	}
	vid := parsed.Content[2].(*VideoBlock)
	if vid.URL != "http://old.com/vid.mp4" {
		t.Fatalf("unexpected video: %+v", vid)
	}
}

func TestMsgJSON_ToolUseBlock_RawInput(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Content(
		&ToolUseBlock{ID: "t1", Name: "calc", Input: map[string]any{"x": 1}, RawInput: `{"x":1}`},
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	tu := parsed.Content[0].(*ToolUseBlock)
	if tu.RawInput != `{"x":1}` {
		t.Fatalf("unexpected raw_input: %s", tu.RawInput)
	}
}

func TestMsgJSON_ToolResultBlock_ExtraFields(t *testing.T) {
	b := NewToolResultBlock("tu1", []ContentBlock{NewTextBlock("ok")}, false)
	b.ID = "tr1"
	b.Name = "calc"
	b.State = "completed"

	msg := NewMsg().Role(RoleTool).Content(b).Build()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	tr := parsed.Content[0].(*ToolResultBlock)
	// Cross-lang: Python ToolResultBlock uses 'id' for tool_call_id, so Go's
	// ID field is mapped to ToolUseID on deserialization.
	if tr.ToolUseID != "tu1" || tr.Name != "calc" || tr.State != "completed" {
		t.Fatalf("unexpected tool result block: %+v", tr)
	}
}

func TestMsgJSON_ToolCallFormat(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Content(
		&ToolUseBlock{ID: "tc1", Name: "calculator", Input: map[string]any{"expr": "1+1"}, RawInput: `{"expr":"1+1"}`},
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw failed: %v", err)
	}
	content := raw["content"].([]any)
	block := content[0].(map[string]any)
	if block["type"] != "tool_call" {
		t.Fatalf("expected type tool_call, got %v", block["type"])
	}
	if block["name"] != "calculator" {
		t.Fatalf("unexpected name: %v", block["name"])
	}
	if block["input"] != `{"expr":"1+1"}` {
		t.Fatalf("unexpected input: %v", block["input"])
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	tu := parsed.Content[0].(*ToolUseBlock)
	if tu.Name != "calculator" || tu.RawInput != `{"expr":"1+1"}` {
		t.Fatalf("unexpected tool use block: %+v", tu)
	}
}

func TestMsgJSON_ToolResultOutputField(t *testing.T) {
	msg := NewMsg().Role(RoleTool).Content(
		NewToolResultBlock("tc1", []ContentBlock{NewTextBlock("result=2")}, false),
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw failed: %v", err)
	}
	content := raw["content"].([]any)
	block := content[0].(map[string]any)
	if block["type"] != "tool_result" {
		t.Fatalf("expected type tool_result, got %v", block["type"])
	}
	output, ok := block["output"].([]any)
	if !ok || len(output) != 1 {
		t.Fatalf("expected output array with 1 element, got %v", block["output"])
	}
}

func TestMsgJSON_CrossLangPyV2ToolResult(t *testing.T) {
	// Simulate a Python v2-generated ToolResultBlock JSON.
	pyJSON := `{
		"id":"1","role":"tool","created_at":"2024-01-01T00:00:00Z",
		"content":[
			{"type":"tool_result","id":"tc1","name":"calculator","output":[{"type":"text","text":"42"}],"is_error":false}
		]
	}`
	var parsed Msg
	if err := json.Unmarshal([]byte(pyJSON), &parsed); err != nil {
		t.Fatalf("unmarshal py format failed: %v", err)
	}
	if len(parsed.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(parsed.Content))
	}
	tr := parsed.Content[0].(*ToolResultBlock)
	if tr.ToolUseID != "tc1" || tr.Name != "calculator" {
		t.Fatalf("unexpected tool result: %+v", tr)
	}
	if len(tr.Content) != 1 {
		t.Fatalf("expected 1 sub-content, got %d", len(tr.Content))
	}
}

func TestMsgJSON_ImageBlock_DefaultMimeType(t *testing.T) {
	msg := NewMsg().Role(RoleUser).Content(
		NewImageBlock("http://x.com/img.png", "", ""),
	).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	img := parsed.Content[0].(*ImageBlock)
	if img.URL != "http://x.com/img.png" || img.MimeType != "image/png" {
		t.Fatalf("unexpected image block: %+v", img)
	}
}

func TestMsgJSON_SpecialCharactersInText(t *testing.T) {
	text := "Hello \n\t<>\"'& 你好 世界 🌍"
	msg := NewMsg().Role(RoleUser).TextContent(text).Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed.GetTextContent() != text {
		t.Fatalf("unexpected text: %q", parsed.GetTextContent())
	}
}

func TestMsgJSON_FinishedAtAndUsage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	usage := &TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	msg := NewMsg().Role(RoleAssistant).TextContent("hi").Build()
	msg.FinishedAt = &now
	msg.Usage = usage

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw failed: %v", err)
	}
	if raw["finished_at"] == nil {
		t.Fatal("expected finished_at in JSON")
	}
	if raw["usage"] == nil {
		t.Fatal("expected usage in JSON")
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed.FinishedAt == nil {
		t.Fatal("expected FinishedAt after round-trip")
	}
	if !parsed.FinishedAt.Equal(now) {
		t.Fatalf("expected FinishedAt %v, got %v", now, *parsed.FinishedAt)
	}
	if parsed.Usage == nil {
		t.Fatal("expected Usage after round-trip")
	}
	if parsed.Usage.PromptTokens != 10 || parsed.Usage.CompletionTokens != 5 || parsed.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", parsed.Usage)
	}
}

func TestMsgJSON_WithoutFinishedAtAndUsage(t *testing.T) {
	msg := NewMsg().Role(RoleUser).TextContent("hello").Build()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw failed: %v", err)
	}
	if raw["finished_at"] != nil {
		t.Fatal("expected no finished_at when nil")
	}
	if raw["usage"] != nil {
		t.Fatal("expected no usage when nil")
	}

	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed.FinishedAt != nil {
		t.Fatal("expected nil FinishedAt")
	}
	if parsed.Usage != nil {
		t.Fatal("expected nil Usage")
	}
}
