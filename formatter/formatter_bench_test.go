package formatter

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func buildTextMsgs(n int) []*message.Msg {
	msgs := make([]*message.Msg, 0, n)
	for i := 0; i < n; i++ {
		role := message.RoleUser
		if i%2 == 1 {
			role = message.RoleAssistant
		}
		msgs = append(msgs, message.NewMsg().Role(role).TextContent("hello world, this is a sample message for benchmarking").Build())
	}
	return msgs
}

func buildToolSpecs(n int) []model.ToolSpec {
	specs := make([]model.ToolSpec, n)
	for i := 0; i < n; i++ {
		specs[i] = model.ToolSpec{
			Name:        "tool_" + string(rune('a'+i%26)),
			Description: "A sample tool for benchmarking purposes that does something useful",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input":       map[string]any{"type": "string"},
					"temperature": map[string]any{"type": "number"},
					"max_tokens":  map[string]any{"type": "integer"},
				},
				"required": []string{"input"},
			},
		}
	}
	return specs
}

func buildMultimodalMsg() *message.Msg {
	return message.NewMsg().
		Role(message.RoleUser).
		TextContent("Describe this image").
		Content(message.NewImageBlock("https://example.com/photo.jpg", "", "")).
		Build()
}

func buildToolResultMsgs(toolCallID string) *message.Msg {
	return message.NewMsg().
		Role(message.RoleTool).
		Content(message.NewToolResultBlock(toolCallID, []message.ContentBlock{message.NewTextBlock("result: success with some output")}, false)).
		Build()
}

func buildToolCallMsgs(toolCallID, toolName string) *message.Msg {
	return message.NewMsg().
		Role(message.RoleAssistant).
		Content(message.NewToolUseBlock(toolCallID, toolName, map[string]any{"input": "benchmark", "param": 42})).
		Build()
}

func buildThinkingMsg() *message.Msg {
	return message.NewMsg().
		Role(message.RoleAssistant).
		Content(message.NewThinkingBlock("Let me think about this carefully. The user wants to know... I should consider... Actually, the answer is...", "")).
		Build()
}

// ================================================================
// OpenAI Formatter Benchmarks
// ================================================================

func BenchmarkOpenAI_FormatMessages_1Msg(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := buildTextMsgs(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatMessages_10Msgs(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := buildTextMsgs(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatMessages_50Msgs(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := buildTextMsgs(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatMessages_Multimodal(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := []*message.Msg{buildMultimodalMsg()}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatMessages_ToolCall(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := []*message.Msg{buildToolCallMsgs("call_1", "search")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatMessages_ToolResult(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := []*message.Msg{buildToolResultMsgs("call_1")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatMessages_Thinking(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := []*message.Msg{buildThinkingMsg()}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

func BenchmarkOpenAI_FormatTools_1(b *testing.B) {
	f := NewOpenAIFormatter()
	specs := buildToolSpecs(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatToolsTyped(specs)
	}
}

func BenchmarkOpenAI_FormatTools_5(b *testing.B) {
	f := NewOpenAIFormatter()
	specs := buildToolSpecs(5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatToolsTyped(specs)
	}
}

func BenchmarkOpenAI_FormatTools_20(b *testing.B) {
	f := NewOpenAIFormatter()
	specs := buildToolSpecs(20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatToolsTyped(specs)
	}
}

func BenchmarkOpenAI_FormatMessages_Mixed(b *testing.B) {
	f := NewOpenAIFormatter()
	msgs := []*message.Msg{
		buildTextMsgs(1)[0],
		buildMultimodalMsg(),
		buildToolCallMsgs("call_1", "search"),
		buildToolResultMsgs("call_1"),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

// ================================================================
// Anthropic Formatter Benchmarks
// ================================================================

func BenchmarkAnthropic_FormatMessages_10Msgs(b *testing.B) {
	f := NewAnthropicFormatter()
	msgs := buildTextMsgs(10)
	msgs = append([]*message.Msg{
		message.NewMsg().Role(message.RoleSystem).TextContent("You are a helpful assistant.").Build(),
	}, msgs...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessages(msgs)
	}
}

// ================================================================
// Gemini Formatter Benchmarks
// ================================================================

func BenchmarkGemini_FormatContents_10Msgs(b *testing.B) {
	f := NewGeminiFormatter()
	msgs := buildTextMsgs(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatContents(msgs)
	}
}

// ================================================================
// DashScope Formatter (alias of OpenAIFormatter) Benchmarks
// ================================================================

func BenchmarkDashScope_FormatMessages_10Msgs(b *testing.B) {
	f := NewDashScopeFormatter()
	msgs := buildTextMsgs(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatMessagesTyped(msgs)
	}
}

// ================================================================
// Thinking Extraction Benchmarks
// ================================================================

func BenchmarkExtractThinkingBlocks_NoThinking(b *testing.B) {
	builder := message.NewMsg().Role(message.RoleAssistant)
	content := "This is a normal response with no thinking tags at all."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractThinkingBlocks(builder, content)
	}
}

func BenchmarkExtractThinkingBlocks_WithThinking(b *testing.B) {
	content := "Before thinking <think>This is a detailed chain-of-thought reasoning that goes on for several sentences explaining the step-by-step logic behind the answer.</think> After thinking, the answer is 42."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := message.NewMsg().Role(message.RoleAssistant)
		extractThinkingBlocks(builder, content)
	}
}

func BenchmarkExtractThinkingBlocks_MultipleThinking(b *testing.B) {
	content := "Start <think>First thought: consider the problem.</think> Middle <thinking>Second thought: evaluate options.</thinking> End"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := message.NewMsg().Role(message.RoleAssistant)
		extractThinkingBlocks(builder, content)
	}
}
