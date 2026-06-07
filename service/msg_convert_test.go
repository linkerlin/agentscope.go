package service

import (
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

func TestMsgToStored_TextOnly(t *testing.T) {
	m := message.NewMsg().Role(message.RoleUser).TextContent("hello").Build()
	sm := MsgToStored(m, "s1")
	if sm.ID != m.ID {
		t.Fatalf("id mismatch")
	}
	if sm.SessionID != "s1" {
		t.Fatalf("session_id mismatch")
	}
	if sm.Role != "user" {
		t.Fatalf("role mismatch: %s", sm.Role)
	}
	if sm.Content != "hello" {
		t.Fatalf("content mismatch: %s", sm.Content)
	}
	if sm.Blocks == "" {
		t.Fatal("expected Blocks to be set for text content")
	}
}

func TestMsgToStored_WithToolCall(t *testing.T) {
	m := message.NewMsg().Role(message.RoleAssistant).
		Content(message.NewToolUseBlock("tc1", "calc", map[string]any{"x": 1})).
		Build()
	sm := MsgToStored(m, "s1")
	if sm.Content != "" {
		t.Fatalf("expected empty text content, got %q", sm.Content)
	}
	if sm.Blocks == "" {
		t.Fatal("expected Blocks for tool_use content")
	}
}

func TestMsgToStored_WithFinishedAtAndUsage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	usage := &message.TokenUsage{PromptTokens: 10, CompletionTokens: 5}
	m := message.NewMsg().Role(message.RoleAssistant).TextContent("hi").Build()
	m.FinishedAt = &now
	m.Usage = usage

	sm := MsgToStored(m, "s1")
	if sm.FinishedAt == nil || !sm.FinishedAt.Equal(now) {
		t.Fatalf("expected FinishedAt to be preserved")
	}
}

func TestStoredToMsg_TextOnly(t *testing.T) {
	sm := &StoredMessage{
		ID:        "m1",
		SessionID: "s1",
		Role:      "user",
		Content:   "hello",
		CreatedAt: time.Now(),
	}
	m, err := StoredToMsg(sm)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "m1" {
		t.Fatalf("id mismatch")
	}
	if m.Role != message.RoleUser {
		t.Fatalf("role mismatch")
	}
	if m.GetTextContent() != "hello" {
		t.Fatalf("text mismatch: %s", m.GetTextContent())
	}
}

func TestStoredToMsg_WithBlocks(t *testing.T) {
	m := message.NewMsg().Role(message.RoleAssistant).
		Content(
			message.NewTextBlock("result: "),
			message.NewToolUseBlock("tc1", "calc", map[string]any{"x": 1}),
		).Build()
	sm := MsgToStored(m, "s1")

	restored, err := StoredToMsg(sm)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(restored.Content))
	}
	if restored.GetTextContent() != "result: " {
		t.Fatalf("text mismatch: %s", restored.GetTextContent())
	}
	calls := restored.GetToolUseCalls()
	if len(calls) != 1 || calls[0].Name != "calc" {
		t.Fatalf("tool call mismatch: %+v", calls)
	}
}

func TestStoredToMsg_WithFinishedAt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	sm := &StoredMessage{
		ID:         "m1",
		Role:       "assistant",
		Content:    "hi",
		CreatedAt:  time.Now(),
		FinishedAt: &now,
	}
	m, err := StoredToMsg(sm)
	if err != nil {
		t.Fatal(err)
	}
	if m.FinishedAt == nil || !m.FinishedAt.Equal(now) {
		t.Fatal("expected FinishedAt to round-trip")
	}
}

func TestStoredToMsg_BackwardCompatNoBlocks(t *testing.T) {
	// Old data: Content has text, Blocks is empty
	sm := &StoredMessage{
		ID:        "m1",
		Role:      "assistant",
		Content:   "old message",
		CreatedAt: time.Now(),
	}
	m, err := StoredToMsg(sm)
	if err != nil {
		t.Fatal(err)
	}
	if m.GetTextContent() != "old message" {
		t.Fatalf("backward compat text mismatch: %s", m.GetTextContent())
	}
}

func TestRoundTrip_MsgToStoredToMsg(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	m := message.NewMsg().
		Role(message.RoleAssistant).
		Name("agent").
		TextContent("Hello ").
		Content(message.NewThinkingBlock("reasoning", "")).
		Content(message.NewToolUseBlock("tc1", "calc", map[string]any{"expr": "1+1"})).
		Metadata("key", "value").
		Build()
	m.FinishedAt = &now
	m.Usage = &message.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}

	sm := MsgToStored(m, "s1")
	restored, err := StoredToMsg(sm)
	if err != nil {
		t.Fatal(err)
	}

	if restored.ID != m.ID {
		t.Fatalf("id mismatch")
	}
	if restored.Role != m.Role {
		t.Fatalf("role mismatch")
	}
	if restored.Name != m.Name {
		t.Fatalf("name mismatch")
	}
	if restored.GetTextContent() != "Hello " {
		t.Fatalf("text mismatch: %s", restored.GetTextContent())
	}
	if restored.GetThinkingContent() != "reasoning" {
		t.Fatalf("thinking mismatch: %s", restored.GetThinkingContent())
	}
	calls := restored.GetToolUseCalls()
	if len(calls) != 1 || calls[0].Name != "calc" {
		t.Fatalf("tool call mismatch")
	}
	if restored.Metadata["key"] != "value" {
		t.Fatal("metadata mismatch")
	}
	if restored.FinishedAt == nil || !restored.FinishedAt.Equal(now) {
		t.Fatal("finished_at mismatch")
	}
	if restored.Usage == nil || restored.Usage.TotalTokens != 15 {
		t.Fatalf("usage mismatch: %+v", restored.Usage)
	}
}
