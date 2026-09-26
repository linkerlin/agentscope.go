package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

// TestReActAgent_FinalMessageCarriesReplyUsage verifies the final message
// carries the whole reply's accumulated usage, not just the last model call
// (E8c).
func TestReActAgent_FinalMessageCarriesReplyUsage(t *testing.T) {
	echoTool := tool.NewFunctionTool("echo", "echo", map[string]any{"type": "object"}, func(ctx context.Context, input map[string]any) (*tool.Response, error) {
		return tool.NewTextResponse("done"), nil
	})
	toolCallMsg := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", "echo", map[string]any{"x": 1}),
	).Build()
	toolCallMsg.Metadata["usage"] = model.ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	finalMsg := message.NewMsg().Role(message.RoleAssistant).TextContent("final").Build()
	finalMsg.Metadata["usage"] = model.ChatUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30}

	m := &mockToolModel{name: "m", responses: []*message.Msg{toolCallMsg, finalMsg}}
	a, err := Builder().Name("Test").Model(m).Tools(echoTool).Build()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("hi").Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage == nil {
		t.Fatal("expected final message to carry usage")
	}
	if resp.Usage.TotalTokens != 45 {
		t.Fatalf("expected accumulated 45 tokens, got %+v", resp.Usage)
	}
	if resp.Usage.PromptTokens != 30 || resp.Usage.CompletionTokens != 15 {
		t.Fatalf("unexpected accumulated usage: %+v", resp.Usage)
	}
}

// TestUsageTrackingModel_Accumulates verifies background LLM spend is captured.
func TestUsageTrackingModel_Accumulates(t *testing.T) {
	inner := &mockChatModel{name: "m"}
	tracker := &usageTrackingModel{ChatModel: inner}
	msg, err := tracker.Chat(context.Background(), []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// mockChatModel without usage configured attaches no usage record.
	if got := tracker.totalUsage(); got.TotalTokens != 0 {
		t.Fatalf("expected zero usage, got %+v", got)
	}
	_ = msg
}

// TestChatUsage_Add_CacheFields ensures cache counters survive accumulation.
func TestChatUsage_Add_CacheFields(t *testing.T) {
	a := model.ChatUsage{PromptTokens: 100, CachedPromptTokens: 60, CacheCreationTokens: 10, TotalTokens: 120}
	b := model.ChatUsage{PromptTokens: 50, CachedPromptTokens: 20, CacheCreationTokens: 5, TotalTokens: 60}
	got := a.Add(b)
	if got.CachedPromptTokens != 80 || got.CacheCreationTokens != 15 {
		t.Fatalf("cache counters lost in Add: %+v", got)
	}
	if got.PromptTokens != 150 || got.TotalTokens != 180 {
		t.Fatalf("base counters wrong: %+v", got)
	}
}
