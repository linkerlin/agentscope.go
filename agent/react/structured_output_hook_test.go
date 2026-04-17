package react

import (
	"context"
	"testing"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

func TestStructuredOutputHook_PreReasoning_ForcesToolChoice(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderToolChoice, nil, mem)

	reminder := createReminderMessage(ReminderToolChoice)
	e := &hook.PreReasoningEvent{
		Messages: []*message.Msg{message.NewMsg().Role(message.RoleUser).TextContent("hi").Build(), reminder},
	}
	h.handlePreReasoning(e)

	if e.ChatOpts == nil || len(e.ChatOpts) == 0 {
		t.Fatal("expected ChatOpts to be set")
	}
	opts := &model.ChatOptions{}
	for _, o := range e.ChatOpts {
		o(opts)
	}
	if opts.ToolChoice == nil || opts.ToolChoice.Function != generateResponseToolName {
		t.Fatalf("expected tool_choice forced to %s, got %+v", generateResponseToolName, opts.ToolChoice)
	}
}

func TestStructuredOutputHook_PostReasoning_GotoReasoning(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)

	resp := message.NewMsg().Role(message.RoleAssistant).TextContent("plain text").Build()
	e := &hook.PostReasoningEvent{Response: resp}
	res, err := h.handlePostReasoning(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.GotoReasoning {
		t.Fatal("expected goto reasoning")
	}
	if len(res.GotoReasoningMsgs) != 1 {
		t.Fatalf("expected 1 reminder message, got %d", len(res.GotoReasoningMsgs))
	}
}

func TestStructuredOutputHook_PostReasoning_NoGotoWhenToolUse(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)

	resp := message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", generateResponseToolName, map[string]any{}),
	).Build()
	e := &hook.PostReasoningEvent{Response: resp}
	res, err := h.handlePostReasoning(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil && res.GotoReasoning {
		t.Fatal("expected no goto reasoning when tool use present")
	}
}

func TestStructuredOutputHook_PostReasoning_MaxRetries(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)
	h.retryCount = maxStructuredRetries

	resp := message.NewMsg().Role(message.RoleAssistant).TextContent("plain text").Build()
	e := &hook.PostReasoningEvent{Response: resp}
	res, err := h.handlePostReasoning(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil && res.GotoReasoning {
		t.Fatal("expected no goto reasoning after max retries")
	}
}

func TestStructuredOutputHook_PostActing_StopAgent(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)

	resultMsg := message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock(`{"ok":true}`)}, false),
	).Build()
	e := &hook.PostActingEvent{
		ToolName:  generateResponseToolName,
		ResultMsg: resultMsg,
	}
	res, err := h.handlePostActing(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.StopAgent {
		t.Fatal("expected stop agent")
	}
	if !h.completed {
		t.Fatal("expected completed to be true")
	}
}

func TestStructuredOutputHook_PostActing_IgnoresOtherTools(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)

	resultMsg := message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock(`{"ok":true}`)}, false),
	).Build()
	e := &hook.PostActingEvent{
		ToolName:  "search",
		ResultMsg: resultMsg,
	}
	res, err := h.handlePostActing(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil && res.StopAgent {
		t.Fatal("expected no stop agent for unrelated tool")
	}
}

func TestStructuredOutputHook_PostCall_CompressesMemory(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	_ = mem.Add(message.NewMsg().Role(message.RoleUser).TextContent("hello").Build())
	_ = mem.Add(createReminderMessage(ReminderPrompt))
	_ = mem.Add(message.NewMsg().Role(message.RoleAssistant).Content(
		message.NewToolUseBlock("call_1", generateResponseToolName, map[string]any{}),
	).Build())
	_ = mem.Add(message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock(`{"response_msg":"final"}`)}, false),
	).Build())

	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)
	h.completed = true
	recent, _ := mem.GetRecent(1)
	h.resultMsg = recent[0]
	h.handlePostCall()

	msgs, _ := mem.GetAll()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after compression, got %d", len(msgs))
	}
	if msgs[0].GetTextContent() != "hello" {
		t.Fatalf("expected first msg to be user hello, got %s", msgs[0].GetTextContent())
	}
	if msgs[1].Role != message.RoleAssistant {
		t.Fatalf("expected final assistant msg, got %v", msgs[1].Role)
	}
}

func TestStructuredOutputHook_Priority(t *testing.T) {
	h := NewStructuredOutputHook(ReminderPrompt, nil, nil)
	if h.Priority() != 50 {
		t.Fatalf("expected priority 50, got %d", h.Priority())
	}
}

func TestStructuredOutputHook_OnEvent_PostCall(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)
	h.completed = true

	_, err := h.OnEvent(context.Background(), &hook.HookContext{Point: hook.HookPostCall})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStructuredOutputHook_OnStreamEvent(t *testing.T) {
	mem := memory.NewInMemoryMemory()
	h := NewStructuredOutputHook(ReminderPrompt, nil, mem)

	// PreReasoning
	pre := &hook.PreReasoningEvent{Messages: []*message.Msg{createReminderMessage(ReminderPrompt)}}
	_, _ = h.OnStreamEvent(context.Background(), pre)

	// PostReasoning -> goto
	res, _ := h.OnStreamEvent(context.Background(), &hook.PostReasoningEvent{
		Response: message.NewMsg().Role(message.RoleAssistant).TextContent("ok").Build(),
	})
	if res == nil || !res.GotoReasoning {
		t.Fatal("expected goto reasoning from OnStreamEvent")
	}
}


func TestStructuredOutputHook_collectMetadata_NoUsage(t *testing.T) {
	h := NewStructuredOutputHook(ReminderPrompt, nil, nil)
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleAssistant).TextContent("x").Build(),
	}
	h.collectMetadata(msgs)
	if h.aggregatedUsage.TotalTokens != 0 {
		t.Fatalf("expected zero usage, got %+v", h.aggregatedUsage)
	}
}

func TestStructuredOutputHook_extractFinalResponseMsg_Fallback(t *testing.T) {
	h := NewStructuredOutputHook(ReminderPrompt, nil, nil)
	h.resultMsg = message.NewMsg().Role(message.RoleTool).Content(
		message.NewToolResultBlock("call_1", []message.ContentBlock{message.NewTextBlock(`plain text`)}, false),
	).Build()
	msg := h.extractFinalResponseMsg()
	if msg == nil || msg.GetTextContent() != "plain text" {
		t.Fatalf("unexpected fallback msg: %v", msg)
	}
}

func TestStructuredOutputHook_mergeCollectedMetadata_Thinking(t *testing.T) {
	h := NewStructuredOutputHook(ReminderPrompt, nil, nil)
	h.aggregatedThinking = &message.ThinkingBlock{Thinking: "think"}
	msg := message.NewMsg().Role(message.RoleAssistant).TextContent("hi").Build()
	out := h.mergeCollectedMetadata(msg)
	if len(out.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(out.Content))
	}
	if _, ok := out.Content[0].(*message.ThinkingBlock); !ok {
		t.Fatal("expected thinking block first")
	}
}

func TestMapToMsg(t *testing.T) {
	m := map[string]any{
		"role":    "user",
		"content": "hello",
		"metadata": map[string]any{
			"k": "v",
		},
	}
	msg := mapToMsg(m)
	if msg.Role != message.RoleUser || msg.GetTextContent() != "hello" {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}
