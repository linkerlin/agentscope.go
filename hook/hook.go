package hook

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// HookPoint defines when during agent execution a hook fires
type HookPoint string

const (
	// HookBeforeModel (pre_model): messages formatted, before model API call.
	HookBeforeModel HookPoint = "before_model"
	// HookAfterModel (post_model): raw model output, before parsing downstream.
	HookAfterModel   HookPoint = "after_model"
	HookBeforeTool   HookPoint = "before_tool"
	HookAfterTool    HookPoint = "after_tool"
	HookBeforeFinish HookPoint = "before_finish"
	HookPreCall      HookPoint = "pre_call"
	HookPostCall     HookPoint = "post_call"

	// 高层生命周期 Hook（对齐 Python 版 AgentBase）
	HookPreReply    HookPoint = "pre_reply"
	HookPostReply   HookPoint = "post_reply"
	HookPreObserve  HookPoint = "pre_observe"
	HookPostObserve HookPoint = "post_observe"

	// HookOnError (on_error): any lifecycle error before propagating to caller.
	HookOnError HookPoint = "on_error"
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
	ChatOpts  []model.ChatOption
	Err       error
}

// HookResult allows a hook to modify agent execution
type HookResult struct {
	Override          *message.Msg
	Interrupt         bool
	InjectMessages    []*message.Msg
	StopAgent         bool
	GotoReasoning     bool
	GotoReasoningMsgs []*message.Msg
	// HandleError swallows the error in HookOnError when true (return Override or nil).
	HandleError bool
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
