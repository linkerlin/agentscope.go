package memory

import (
	"context"

	"github.com/linkerlin/agentscope.go/hook"
)

// ReMeHook 在 HookBeforeModel 时调用 ReMeMemory.PreReasoningPrepare 替换 Messages
type ReMeHook struct {
	Mem ReMeMemory
}

// NewReMeHook 创建 Hook；mem 需实现 PreReasoningPrepare（如 *ReMeFileMemory）
func NewReMeHook(mem ReMeMemory) hook.Hook {
	return &ReMeHook{Mem: mem}
}

func (h *ReMeHook) OnEvent(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
	if h == nil || h.Mem == nil || hCtx == nil {
		return nil, nil
	}
	if hCtx.Point != hook.HookBeforeModel {
		return nil, nil
	}
	out, _, err := h.Mem.PreReasoningPrepare(ctx, hCtx.Messages)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}
	return &hook.HookResult{InjectMessages: out}, nil
}
