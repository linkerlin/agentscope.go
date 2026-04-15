package hook

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// ErrInterrupted 表示 StreamHook 请求中断当前执行
var ErrInterrupted = errors.New("hook: stream interrupted")

// EventType 流式/事件驱动 Hook 的事件类型（对齐演进方案 5.3）
type EventType string

const (
	EventPreCall        EventType = "pre_call"
	EventPostCall       EventType = "post_call"
	EventPreReasoning   EventType = "pre_reasoning"
	EventPostReasoning  EventType = "post_reasoning"
	EventReasoningChunk EventType = "reasoning_chunk"
	EventPreActing      EventType = "pre_acting"
	EventPostActing     EventType = "post_acting"
	EventActingChunk    EventType = "acting_chunk"
	EventError          EventType = "error"
)

// Event 事件基础接口
type Event interface {
	EventType() EventType
	Timestamp() time.Time
	AgentName() string
}

// BaseEvent 公共字段
type BaseEvent struct {
	Type      EventType
	Ts        time.Time
	Agent     string
	Iteration int
}

func (e BaseEvent) EventType() EventType   { return e.Type }
func (e BaseEvent) Timestamp() time.Time { return e.Ts }
func (e BaseEvent) AgentName() string      { return e.Agent }

// PreReasoningEvent 模型推理前（可携带将送入模型的历史快照）
type PreReasoningEvent struct {
	BaseEvent
	Messages  []*message.Msg
	ModelName string
	ChatOpts  []model.ChatOption
}

// PostReasoningEvent 单次模型输出完成后（聚合后的 assistant 消息）
type PostReasoningEvent struct {
	BaseEvent
	Messages []*message.Msg
	Response *message.Msg
}

// ReasoningChunkEvent 流式推理片段（文本增量）
type ReasoningChunkEvent struct {
	BaseEvent
	Messages []*message.Msg
	Chunk    string
}

// PreActingEvent 工具执行前
type PreActingEvent struct {
	BaseEvent
	Messages  []*message.Msg
	ToolName  string
	ToolInput map[string]any
}

// PostActingEvent 工具执行后
type PostActingEvent struct {
	BaseEvent
	Messages  []*message.Msg
	ToolName  string
	ToolInput map[string]any
	Result    any
	Err       error
	ResultMsg *message.Msg
}

// ActingChunkEvent 工具执行过程的流式片段（可选，用于长耗时工具的进度）
type ActingChunkEvent struct {
	BaseEvent
	ToolName string
	Chunk    string
}

// ErrorEvent 执行错误
type ErrorEvent struct {
	BaseEvent
	Err error
}

// StreamHookResult 流式 Hook 可请求中断、停止或重新推理当前轮次（由 ReActAgent 解释）
type StreamHookResult struct {
	Interrupt         bool
	StopAgent         bool
	GotoReasoning     bool
	GotoReasoningMsgs []*message.Msg
	Override          *message.Msg
}

// StreamHook 第二套可选接口：接收结构化事件（含流式 chunk），与经典 Hook 并存
type StreamHook interface {
	OnStreamEvent(ctx context.Context, ev Event) (*StreamHookResult, error)
}

// StreamHookFunc 将函数适配为 StreamHook
type StreamHookFunc func(ctx context.Context, ev Event) (*StreamHookResult, error)

func (f StreamHookFunc) OnStreamEvent(ctx context.Context, ev Event) (*StreamHookResult, error) {
	return f(ctx, ev)
}

// StreamPriorityOf 返回 StreamHook 优先级；实现 Prioritized 时取其值，否则 DefaultPriority
func StreamPriorityOf(h StreamHook) int {
	if h == nil {
		return DefaultPriority
	}
	if ph, ok := h.(Prioritized); ok {
		return ph.Priority()
	}
	return DefaultPriority
}

// SortStreamHooks 按优先级稳定排序 StreamHook
func SortStreamHooks(hooks []StreamHook) []StreamHook {
	if len(hooks) <= 1 {
		out := make([]StreamHook, len(hooks))
		copy(out, hooks)
		return out
	}
	type item struct {
		h StreamHook
		i int
	}
	items := make([]item, len(hooks))
	for i, h := range hooks {
		items[i] = item{h: h, i: i}
	}
	sort.SliceStable(items, func(i, j int) bool {
		pi, pj := StreamPriorityOf(items[i].h), StreamPriorityOf(items[j].h)
		if pi != pj {
			return pi < pj
		}
		return items[i].i < items[j].i
	})
	out := make([]StreamHook, len(items))
	for i := range items {
		out[i] = items[i].h
	}
	return out
}
