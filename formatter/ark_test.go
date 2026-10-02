// formatter/ark_test.go — 20.3: the Ark formatter drops NO content block
// (reasoning_content included) and maps stream deltas with thinking flags.
package formatter

import (
	"encoding/json"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// TestArkParseArkMessage_AllBlocksSurvive: reasoning_content + content +
// tool_calls all become message content, in that order.
func TestArkParseArkMessage_AllBlocksSurvive(t *testing.T) {
	var m ArkAssistantMessage
	raw := `{
		"content": "答案文本",
		"reasoning_content": "推理文本",
		"tool_calls": [
			{"id": "c1", "type": "function", "function": {"name": "f", "arguments": "{\"a\":1}"}},
			{"id": "c2", "type": "function", "function": {"name": "g", "arguments": "{}"}}
		]
	}`
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	f := NewArkFormatter()
	msg := f.ParseArkMessage(&m)
	if got := msg.GetThinkingContent(); got != "推理文本" {
		t.Fatalf("thinking dropped: %q", got)
	}
	if got := msg.GetTextContent(); got != "答案文本" {
		t.Fatalf("text dropped: %q", got)
	}
	if calls := msg.GetToolUseCalls(); len(calls) != 2 || calls[0].Name != "f" || calls[1].Name != "g" || calls[0].Input["a"] != float64(1) {
		t.Fatalf("tool calls dropped: %+v", calls)
	}

	// Order: thinking block precedes text and tools in the raw block list.
	for i, b := range msg.Content {
		if _, isThinking := b.(*message.ThinkingBlock); isThinking && i != 0 {
			t.Fatalf("thinking must be the first block, got position %d", i)
		}
	}
}

// TestArkParseArkMessage_ThinkingOnly: a reasoning-only message (content
// empty) still carries the thinking block.
func TestArkParseArkMessage_ThinkingOnly(t *testing.T) {
	f := NewArkFormatter()
	msg := f.ParseArkMessage(&ArkAssistantMessage{ReasoningContent: "只有思考"})
	if msg.GetThinkingContent() != "只有思考" || msg.GetTextContent() != "" {
		t.Fatalf("msg: %+v", msg)
	}
}

// TestArkParseArkStreamDelta: reasoning deltas are flagged IsThinking,
// content deltas are plain, tool calls arrive as content blocks.
func TestArkParseArkStreamDelta(t *testing.T) {
	var ev ArkStreamEvent
	raw := `{"choices":[{"index":0,"delta":{
		"reasoning_content":"想","content":"说",
		"tool_calls":[{"id":"t1","type":"function","function":{"name":"f","arguments":"{\"k\":true}"}}]
	}}]}`
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatal(err)
	}
	chunks := NewArkFormatter().ParseArkStreamDelta(&ev)
	if len(chunks) != 3 {
		t.Fatalf("chunks: %d", len(chunks))
	}
	if !chunks[0].IsThinking || chunks[0].Delta != "想" {
		t.Fatalf("thinking chunk: %+v", chunks[0])
	}
	if chunks[1].IsThinking || chunks[1].Delta != "说" {
		t.Fatalf("content chunk: %+v", chunks[1])
	}
	if len(chunks[2].Content) != 1 {
		t.Fatalf("tool chunk: %+v", chunks[2])
	}
	tu, ok := chunks[2].Content[0].(*message.ToolUseBlock)
	if !ok || tu.Name != "f" || tu.Input["k"] != true {
		t.Fatalf("tool block: %+v", chunks[2].Content[0])
	}
}

// TestArkResponseFormat_Shapes: json_object / json_schema / passthrough.
func TestArkResponseFormat_Shapes(t *testing.T) {
	if rf := ArkResponseFormat(&model.ResponseFormat{Type: "json_object"}); rf["type"] != "json_object" {
		t.Fatalf("json_object: %v", rf)
	}
	rf := ArkResponseFormat(&model.ResponseFormat{
		Type:       "json_schema",
		JSONSchema: map[string]any{"type": "object"},
	})
	if rf["type"] != "json_schema" {
		t.Fatalf("json_schema type: %v", rf)
	}
	js := rf["json_schema"].(map[string]any)
	if js["schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("json_schema schema: %v", js)
	}
	// json_schema without a schema object degrades to json_object.
	if rf := ArkResponseFormat(&model.ResponseFormat{Type: "json_schema"}); rf["type"] != "json_object" {
		t.Fatalf("schemaless fallback: %v", rf)
	}
	if ArkResponseFormat(nil) != nil {
		t.Fatal("nil must pass through")
	}
}
