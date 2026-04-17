package skill

import (
	"context"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

// Hook 实现 hook.Hook，在 PreReasoning（BeforeModel）阶段注入 skill prompt
type Hook struct {
	Provider *PromptProvider
}

// NewHook 创建 Skill Hook
func NewHook(provider *PromptProvider) *Hook {
	return &Hook{Provider: provider}
}

// OnEvent 在 BeforeModel 时注入 skill system prompt
func (h *Hook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if hCtx.Point != hook.HookBeforeModel {
		return nil, nil
	}
	prompt := h.Provider.GetSkillPrompt()
	if prompt == "" {
		return nil, nil
	}

	msgs := hCtx.Messages
	sysIdx := -1
	for i, m := range msgs {
		if m.Role == message.RoleSystem {
			sysIdx = i
			break
		}
	}

	if sysIdx >= 0 {
		// 追加到现有 system message
		newMsg := message.NewMsg().Role(message.RoleSystem).
			TextContent(msgs[sysIdx].GetTextContent() + "\n\n" + prompt).Build()
		newMsgs := make([]*message.Msg, len(msgs))
		copy(newMsgs, msgs)
		newMsgs[sysIdx] = newMsg
		return &hook.HookResult{InjectMessages: newMsgs}, nil
	}

	// 在开头插入新 system message
	newMsg := message.NewMsg().Role(message.RoleSystem).TextContent(prompt).Build()
	newMsgs := append([]*message.Msg{newMsg}, msgs...)
	return &hook.HookResult{InjectMessages: newMsgs}, nil
}
