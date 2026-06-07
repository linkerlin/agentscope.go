package message

import (
	"encoding/json"
	"testing"

	"github.com/linkerlin/agentscope.go/event"
)

func TestAppendEvent_Nil(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(nil)
	if len(msg.Content) != 0 {
		t.Fatal("nil event should be no-op")
	}
}

func TestAppendEvent_TextBlock(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewTextBlockStart("r1", 0))
	msg.AppendEvent(event.NewTextBlockDelta("r1", 0, "Hello "))
	msg.AppendEvent(event.NewTextBlockDelta("r1", 0, "world"))
	msg.AppendEvent(event.NewTextBlockEnd("r1", 0))

	if msg.GetTextContent() != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", msg.GetTextContent())
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(msg.Content))
	}
}

func TestAppendEvent_ThinkingBlock(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewThinkingBlockStart("r1", 0))
	msg.AppendEvent(event.NewThinkingBlockDelta("r1", 0, "reasoning..."))
	msg.AppendEvent(event.NewThinkingBlockEnd("r1", 0))

	if msg.GetThinkingContent() != "reasoning..." {
		t.Fatalf("expected 'reasoning...', got %q", msg.GetThinkingContent())
	}
}

func TestAppendEvent_HintBlock(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewHintBlockStart("r1", 0))
	msg.AppendEvent(event.NewHintBlockDelta("r1", 0, "suggestion"))
	msg.AppendEvent(event.NewHintBlockEnd("r1", 0))

	if msg.GetHintContent() != "suggestion" {
		t.Fatalf("expected 'suggestion', got %q", msg.GetHintContent())
	}
}

func TestAppendEvent_MixedTextAndThinking(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewThinkingBlockStart("r1", 0))
	msg.AppendEvent(event.NewThinkingBlockDelta("r1", 0, "think"))
	msg.AppendEvent(event.NewThinkingBlockEnd("r1", 0))
	msg.AppendEvent(event.NewTextBlockStart("r1", 0))
	msg.AppendEvent(event.NewTextBlockDelta("r1", 0, "answer"))
	msg.AppendEvent(event.NewTextBlockEnd("r1", 0))

	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(msg.Content))
	}
	if msg.GetThinkingContent() != "think" {
		t.Fatalf("unexpected thinking: %q", msg.GetThinkingContent())
	}
	if msg.GetTextContent() != "answer" {
		t.Fatalf("unexpected text: %q", msg.GetTextContent())
	}
}

func TestAppendEvent_ToolCall(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewToolCallStart("r1", 0, "tc1", "calculator"))
	msg.AppendEvent(event.NewToolCallDelta("r1", 0, "tc1", `{"expr":"1+1"}`))
	msg.AppendEvent(event.NewToolCallEnd("r1", 0, "tc1"))

	calls := msg.GetToolUseCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].ID != "tc1" || calls[0].Name != "calculator" {
		t.Fatalf("unexpected tool call: %+v", calls[0])
	}
	if calls[0].RawInput != `{"expr":"1+1"}` {
		t.Fatalf("unexpected raw input: %q", calls[0].RawInput)
	}
	expr, ok := calls[0].Input["expr"].(string)
	if !ok || expr != "1+1" {
		t.Fatalf("unexpected parsed input: %+v", calls[0].Input)
	}
}

func TestAppendEvent_ToolCall_MultipleChunks(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewToolCallStart("r1", 0, "tc1", "weather"))
	msg.AppendEvent(event.NewToolCallDelta("r1", 0, "tc1", `{"city":`))
	msg.AppendEvent(event.NewToolCallDelta("r1", 0, "tc1", `"London"}`))
	msg.AppendEvent(event.NewToolCallEnd("r1", 0, "tc1"))

	calls := msg.GetToolUseCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	city, ok := calls[0].Input["city"].(string)
	if !ok || city != "London" {
		t.Fatalf("unexpected parsed input: %+v", calls[0].Input)
	}
}

func TestAppendEvent_ToolResult_Text(t *testing.T) {
	msg := NewMsg().Role(RoleTool).Build()
	msg.AppendEvent(event.NewToolResultStart("r1", 0, "tc1", "calculator"))
	msg.AppendEvent(event.NewToolResultTextDelta("r1", 0, "tc1", "result=42"))
	msg.AppendEvent(event.NewToolResultEnd("r1", 0, "tc1"))

	results := msg.GetToolResults()
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	tr := results[0]
	if tr.ToolUseID != "tc1" || tr.State != "completed" {
		t.Fatalf("unexpected tool result: %+v", tr)
	}
	if len(tr.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(tr.Content))
	}
	if tb, ok := tr.Content[0].(*TextBlock); !ok || tb.Text != "result=42" {
		t.Fatalf("unexpected text block: %+v", tr.Content[0])
	}
}

func TestAppendEvent_ToolResult_Data(t *testing.T) {
	msg := NewMsg().Role(RoleTool).Build()
	msg.AppendEvent(event.NewToolResultStart("r1", 0, "tc1", "image_gen"))
	msg.AppendEvent(event.NewToolResultDataDelta("r1", 0, "tc1", "base64data", "image/png"))
	msg.AppendEvent(event.NewToolResultEnd("r1", 0, "tc1"))

	results := msg.GetToolResults()
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	tr := results[0]
	if len(tr.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(tr.Content))
	}
	img, ok := tr.Content[0].(*ImageBlock)
	if !ok || img.URL != "base64data" || img.MimeType != "image/png" {
		t.Fatalf("unexpected image block: %+v", tr.Content[0])
	}
}

func TestAppendEvent_DataBlock(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewDataBlockStart("r1", 0, "db1", "image/png"))
	msg.AppendEvent(event.NewDataBlockDelta("r1", 0, "db1", "chunk1", "image/png"))
	msg.AppendEvent(event.NewDataBlockDelta("r1", 0, "db1", "chunk2", "image/png"))
	msg.AppendEvent(event.NewDataBlockEnd("r1", 0, "db1"))

	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(msg.Content))
	}
	db, ok := msg.Content[0].(*DataBlock)
	if !ok {
		t.Fatalf("expected DataBlock, got %T", msg.Content[0])
	}
	if db.Source == nil || db.Source.Data != "chunk1chunk2" {
		t.Fatalf("unexpected data: %+v", db.Source)
	}
}

func TestAppendEvent_ReplyEnd_SetsFinishedAt(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	if msg.FinishedAt != nil {
		t.Fatal("expected nil FinishedAt initially")
	}
	msg.AppendEvent(event.NewReplyEnd("r1", "mock"))
	if msg.FinishedAt == nil {
		t.Fatal("expected FinishedAt to be set")
	}
}

func TestAppendEvent_ErrorEvent_NoOp(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewError("r1", nil))
	if len(msg.Content) != 0 {
		t.Fatal("error event should not modify content")
	}
}

func TestAppendEvent_ExceedMaxIters_NoOp(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	msg.AppendEvent(event.NewExceedMaxIters("r1", 5))
	if len(msg.Content) != 0 {
		t.Fatal("exceed_max_iters event should not modify content")
	}
}

func TestAppendEvent_ModelCallEnd_AccumulatesUsage(t *testing.T) {
	msg := NewMsg().Role(RoleAssistant).Build()
	if msg.Usage != nil {
		t.Fatal("expected nil Usage initially")
	}
	msg.AppendEvent(event.NewModelCallEnd("r1", "gpt-4", 10, 5))
	if msg.Usage == nil {
		t.Fatal("expected Usage to be set")
	}
	if msg.Usage.PromptTokens != 10 || msg.Usage.CompletionTokens != 5 || msg.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage after first call: %+v", msg.Usage)
	}
	// Multiple model calls should accumulate
	msg.AppendEvent(event.NewModelCallEnd("r1", "gpt-4", 8, 4))
	if msg.Usage.PromptTokens != 18 || msg.Usage.CompletionTokens != 9 || msg.Usage.TotalTokens != 27 {
		t.Fatalf("unexpected usage after second call: %+v", msg.Usage)
	}
}

func TestAppendEvent_FullStreamReconstruction(t *testing.T) {
	// Simulate a realistic event stream: text + tool call + tool result + final text
	msg := NewMsg().Role(RoleAssistant).Build()

	msg.AppendEvent(event.NewTextBlockStart("r1", 0))
	msg.AppendEvent(event.NewTextBlockDelta("r1", 0, "I'll calculate that."))
	msg.AppendEvent(event.NewTextBlockEnd("r1", 0))

	msg.AppendEvent(event.NewToolCallStart("r1", 0, "tc1", "calc"))
	msg.AppendEvent(event.NewToolCallDelta("r1", 0, "tc1", `{"x":1}`))
	msg.AppendEvent(event.NewToolCallEnd("r1", 0, "tc1"))

	msg.AppendEvent(event.NewToolResultStart("r1", 0, "tc1", "calc"))
	msg.AppendEvent(event.NewToolResultTextDelta("r1", 0, "tc1", "2"))
	msg.AppendEvent(event.NewToolResultEnd("r1", 0, "tc1"))

	msg.AppendEvent(event.NewTextBlockStart("r1", 1))
	msg.AppendEvent(event.NewTextBlockDelta("r1", 1, " The answer is 2."))
	msg.AppendEvent(event.NewTextBlockEnd("r1", 1))

	msg.AppendEvent(event.NewReplyEnd("r1", "mock"))

	if len(msg.Content) != 4 { // text, tool_use, tool_result, text
		t.Fatalf("expected 4 blocks, got %d", len(msg.Content))
	}

	// Verify text concatenation across multiple text blocks
	fullText := ""
	for _, b := range msg.Content {
		if tb, ok := b.(*TextBlock); ok {
			fullText += tb.Text
		}
	}
	if fullText != "I'll calculate that. The answer is 2." {
		t.Fatalf("unexpected full text: %q", fullText)
	}

	if msg.FinishedAt == nil {
		t.Fatal("expected FinishedAt to be set")
	}

	// Verify JSON round-trip
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed Msg
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed.FinishedAt == nil {
		t.Fatal("expected FinishedAt after round-trip")
	}
	if len(parsed.Content) != 4 {
		t.Fatalf("expected 4 blocks after round-trip, got %d", len(parsed.Content))
	}
}
