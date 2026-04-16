package message

import (
	"testing"
)

func TestMsgBuilder(t *testing.T) {
	msg := NewMsg().
		Role(RoleUser).
		Name("alice").
		TextContent("Hello").
		TextContent(" world").
		Metadata("key", "value").
		Build()

	if msg.Role != RoleUser {
		t.Errorf("expected role %s, got %s", RoleUser, msg.Role)
	}
	if msg.Name != "alice" {
		t.Errorf("expected name alice, got %s", msg.Name)
	}
	if msg.GetTextContent() != "Hello world" {
		t.Errorf("unexpected text: %s", msg.GetTextContent())
	}
	if msg.Metadata["key"] != "value" {
		t.Error("metadata not set")
	}
	if msg.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestGetToolUseCalls(t *testing.T) {
	tb := NewToolUseBlock("id1", "my_tool", map[string]any{"arg": 1})
	msg := NewMsg().Role(RoleAssistant).Content(tb).Build()

	calls := msg.GetToolUseCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool use call, got %d", len(calls))
	}
	if calls[0].Name != "my_tool" {
		t.Errorf("unexpected tool name: %s", calls[0].Name)
	}
}

func TestGetToolResults(t *testing.T) {
	rb := NewToolResultBlock("id1", []ContentBlock{NewTextBlock("result")}, false)
	msg := NewMsg().Role(RoleTool).Content(rb).Build()

	results := msg.GetToolResults()
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if results[0].ToolUseID != "id1" {
		t.Errorf("unexpected tool use id: %s", results[0].ToolUseID)
	}
}

func TestBlockTypes(t *testing.T) {
	blocks := []ContentBlock{
		NewTextBlock("text"),
		NewImageBlock("url", "", "image/png"),
		NewAudioBlock("url", "", "audio/mp3"),
		NewVideoBlock("url"),
		NewDataBlock(TypeImage, &Source{Type: SourceTypeURL, URL: "http://x"}),
		NewToolUseBlock("id", "tool", nil),
		NewToolResultBlock("id", nil, false),
		NewThinkingBlock("thoughts", "sig"),
	}

	expected := []BlockType{
		TypeText, TypeImage, TypeAudio, TypeVideo, TypeImage, TypeToolUse, TypeToolResult, TypeThinking,
	}

	for i, b := range blocks {
		if b.BlockType() != expected[i] {
			t.Errorf("block %d: expected %s, got %s", i, expected[i], b.BlockType())
		}
	}
}
