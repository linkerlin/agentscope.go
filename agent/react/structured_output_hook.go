package react

import (
	"context"
	"encoding/json"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
)

const generateResponseToolName = "generate_response"

const (
	maxStructuredRetries = 3
)

// ReminderMode controls how the model is reminded to call generate_response.
type ReminderMode string

const (
	// ReminderToolChoice forces tool_choice to generate_response on retry.
	ReminderToolChoice ReminderMode = "tool_choice"
	// ReminderPrompt uses a plain text reminder.
	ReminderPrompt ReminderMode = "prompt"
)

// StructuredOutputHook aligns with Java's StructuredOutputHook.
// It intercepts ReAct execution to ensure the model calls generate_response
// for structured output generation.
type StructuredOutputHook struct {
	reminderMode ReminderMode
	baseOpts     []model.ChatOption
	memory       memory.Memory

	completed        bool
	resultMsg        *message.Msg
	retryCount       int
	aggregatedUsage  model.ChatUsage
	aggregatedThinking *message.ThinkingBlock
}

// NewStructuredOutputHook creates a new hook.
func NewStructuredOutputHook(reminder ReminderMode, baseOpts []model.ChatOption, mem memory.Memory) *StructuredOutputHook {
	return &StructuredOutputHook{
		reminderMode: reminder,
		baseOpts:     baseOpts,
		memory:       mem,
	}
}

// Priority returns high priority so it runs before other hooks.
func (h *StructuredOutputHook) Priority() int { return 50 }

// OnEvent handles the classic hook point (post_call) for memory compression.
func (h *StructuredOutputHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if hCtx.Point == hook.HookPostCall {
		h.handlePostCall()
	}
	return nil, nil
}

// OnStreamEvent handles streaming events: pre_reasoning, post_reasoning, post_acting.
func (h *StructuredOutputHook) OnStreamEvent(ctx context.Context, ev hook.Event) (*hook.StreamHookResult, error) {
	switch e := ev.(type) {
	case *hook.PreReasoningEvent:
		h.handlePreReasoning(e)
	case *hook.PostReasoningEvent:
		return h.handlePostReasoning(e)
	case *hook.PostActingEvent:
		return h.handlePostActing(e)
	}
	return nil, nil
}

func (h *StructuredOutputHook) handlePreReasoning(e *hook.PreReasoningEvent) {
	if h.reminderMode != ReminderToolChoice || len(e.Messages) == 0 {
		return
	}
	last := e.Messages[len(e.Messages)-1]
	if last != nil && isReminderMessage(last) {
		opts := append([]model.ChatOption{}, h.baseOpts...)
		opts = append(opts, model.WithToolChoice(&model.ToolChoice{
			Mode:     "function",
			Function: generateResponseToolName,
		}))
		e.ChatOpts = opts
	}
}

func (h *StructuredOutputHook) handlePostReasoning(e *hook.PostReasoningEvent) (*hook.StreamHookResult, error) {
	msg := e.Response
	if msg == nil {
		return nil, nil
	}
	hasCall := len(msg.GetToolUseCalls()) > 0
	if !hasCall && h.retryCount < maxStructuredRetries {
		h.retryCount++
		reminder := createReminderMessage(h.reminderMode)
		if err := tool.ValidateToolResultMatch(msg, []*message.Msg{reminder}); err != nil {
			return nil, err
		}
		return &hook.StreamHookResult{
			GotoReasoning:     true,
			GotoReasoningMsgs: []*message.Msg{reminder},
		}, nil
	}
	return nil, nil
}

func (h *StructuredOutputHook) handlePostActing(e *hook.PostActingEvent) (*hook.StreamHookResult, error) {
	if e.ToolName != generateResponseToolName || e.Err != nil {
		return nil, nil
	}
	h.completed = true
	h.resultMsg = e.ResultMsg
	if h.memory != nil {
		msgs, _ := h.memory.GetAll()
		h.collectMetadata(msgs)
	}
	return &hook.StreamHookResult{StopAgent: true}, nil
}

func (h *StructuredOutputHook) handlePostCall() {
	if !h.completed {
		return
	}
	if h.memory == nil {
		return
	}
	original, _ := h.memory.GetAll()
	_ = h.memory.Clear()
	for _, msg := range original {
		if !isStructuredOutputRelated(msg) {
			_ = h.memory.Add(msg)
		}
	}
	final := h.extractFinalResponseMsg()
	if final != nil {
		final = h.mergeCollectedMetadata(final)
		_ = h.memory.Add(final)
	}
}

func (h *StructuredOutputHook) collectMetadata(msgs []*message.Msg) {
	hasUsage := false
	for _, msg := range msgs {
		if !isStructuredOutputRelated(msg) || msg.Role != message.RoleAssistant {
			continue
		}
		if u, ok := msg.Metadata["usage"].(model.ChatUsage); ok {
			hasUsage = true
			h.aggregatedUsage = h.aggregatedUsage.Add(u)
		}
		for _, block := range msg.Content {
			if th, ok := block.(*message.ThinkingBlock); ok {
				h.aggregatedThinking = th
			}
		}
	}
	if !hasUsage {
		h.aggregatedUsage = model.ChatUsage{}
	}
}

func (h *StructuredOutputHook) extractFinalResponseMsg() *message.Msg {
	if h.resultMsg == nil {
		return nil
	}
	for _, block := range h.resultMsg.Content {
		tr, ok := block.(*message.ToolResultBlock)
		if !ok {
			continue
		}
		if tr.IsError {
			continue
		}
		v, ok := tr.Content[0].(*message.TextBlock)
		if !ok {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(v.Text), &payload); err == nil {
			if raw, ok := payload["response_msg"]; ok {
				if m, ok := raw.(map[string]any); ok {
					return mapToMsg(m)
				}
			}
		}
		// Fallback: wrap raw text as assistant message
		return message.NewMsg().Role(message.RoleAssistant).TextContent(v.Text).Build()
	}
	return nil
}

func (h *StructuredOutputHook) mergeCollectedMetadata(msg *message.Msg) *message.Msg {
	if h.aggregatedUsage.TotalTokens > 0 || h.aggregatedUsage.PromptTokens > 0 || h.aggregatedUsage.CompletionTokens > 0 {
		msg.Metadata["usage"] = h.aggregatedUsage
	}
	if h.aggregatedThinking != nil {
		msg.Content = append([]message.ContentBlock{h.aggregatedThinking}, msg.Content...)
	}
	return msg
}

func isReminderMessage(msg *message.Msg) bool {
	if msg == nil || len(msg.Metadata) == 0 {
		return false
	}
	v, ok := msg.Metadata["structured_output_reminder"]
	return ok && v == true
}

func isStructuredOutputRelated(msg *message.Msg) bool {
	if msg == nil {
		return false
	}
	if isReminderMessage(msg) {
		return true
	}
	return hasGenerateResponseTool(msg)
}

func hasGenerateResponseTool(msg *message.Msg) bool {
	for _, block := range msg.Content {
		if tu, ok := block.(*message.ToolUseBlock); ok && tu.Name == generateResponseToolName {
			return true
		}
	}
	for _, block := range msg.Content {
		if tr, ok := block.(*message.ToolResultBlock); ok {
			if tr.ToolUseID == "" {
				continue
			}
			// Heuristic: if any tool result doesn't match, return false
		}
	}
	results := msg.GetToolResults()
	if len(results) == 0 {
		return false
	}
	allMatch := true
	for _, tr := range results {
		// We don't have direct name on ToolResultBlock, so we rely on metadata heuristics
		// or assume all results in a message belong to generate_response when used with this hook.
		_ = tr
	}
	return allMatch
}

func createReminderMessage(mode ReminderMode) *message.Msg {
	return message.NewMsg().
		Role(message.RoleUser).
		Name("system").
		TextContent("Please call the 'generate_response' function to provide your response.").
		Metadata("structured_output_reminder", true).
		Metadata("structured_output_reminder_type", string(mode)).
		Build()
}

func mapToMsg(m map[string]any) *message.Msg {
	msg := message.NewMsg().Role(message.RoleAssistant)
	if role, ok := m["role"].(string); ok {
		msg.Role(message.MsgRole(role))
	}
	if text, ok := m["content"].(string); ok {
		msg.TextContent(text)
	}
	if meta, ok := m["metadata"].(map[string]any); ok {
		for k, v := range meta {
			msg.Metadata(k, v)
		}
	}
	return msg.Build()
}
