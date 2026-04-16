package message

import (
	"encoding/json"
	"testing"
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
	if block["type"] != "image" {
		t.Fatalf("expected type image, got %v", block["type"])
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
	if tr.ID != "tr1" || tr.Name != "calc" || tr.State != "completed" || tr.ToolUseID != "tu1" {
		t.Fatalf("unexpected tool result block: %+v", tr)
	}
}
