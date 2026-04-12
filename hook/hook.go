package hook

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// HookPoint defines when during agent execution a hook fires
type HookPoint string

const (
	HookBeforeModel  HookPoint = "before_model"
	HookAfterModel   HookPoint = "after_model"
	HookBeforeTool   HookPoint = "before_tool"
	HookAfterTool    HookPoint = "after_tool"
	HookBeforeFinish HookPoint = "before_finish"
)

// HookContext contains execution state passed to hooks
type HookContext struct {
	AgentName string
	Point     HookPoint
	Messages  []*message.Msg
	Response  *message.Msg
	ToolName  string
	ToolInput map[string]any
	Metadata  map[string]any
}

// HookResult allows a hook to modify agent execution
type HookResult struct {
	Override       *message.Msg
	Interrupt      bool
	InjectMessages []*message.Msg
}

// Hook is called at various points during agent execution
type Hook interface {
	OnEvent(ctx context.Context, hCtx *HookContext) (*HookResult, error)
}

// HookFunc adapts a plain function to the Hook interface
type HookFunc func(ctx context.Context, hCtx *HookContext) (*HookResult, error)

func (f HookFunc) OnEvent(ctx context.Context, hCtx *HookContext) (*HookResult, error) {
	return f(ctx, hCtx)
}
