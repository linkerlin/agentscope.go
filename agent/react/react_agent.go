package react

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/toolkit"
)

const defaultMaxIterations = 10

// ErrAgentClosed is returned when calling a shut-down agent.
var ErrAgentClosed = errors.New("react agent: agent is closed")

// ReActAgent implements the ReAct (Reasoning + Acting) pattern
type ReActAgent struct {
	*agent.Base

	chatModel     model.ChatModel
	tools         []tool.Tool
	toolkit       *toolkit.Toolkit
	memory        memory.Memory
	maxIterations int
	toolMap       map[string]tool.Tool
}

// ReActAgentBuilder provides a fluent API for constructing ReActAgent
type ReActAgentBuilder struct {
	agentID       string
	name          string
	description   string
	sysPrompt     string
	chatModel     model.ChatModel
	tools         []tool.Tool
	toolkit       *toolkit.Toolkit
	memory        memory.Memory
	maxIterations int
	hooks         []hook.Hook
	streamHooks   []hook.StreamHook
	meta          map[string]any
}

// Builder returns a new ReActAgentBuilder
func Builder() *ReActAgentBuilder {
	return &ReActAgentBuilder{
		maxIterations: defaultMaxIterations,
	}
}

func (b *ReActAgentBuilder) Name(name string) *ReActAgentBuilder {
	b.name = name
	return b
}

// ID 设置持久化用的 Agent 标识；为空时 SaveTo 使用 Name 作为默认 ID
func (b *ReActAgentBuilder) ID(id string) *ReActAgentBuilder {
	b.agentID = id
	return b
}

// Description 设置 Agent 描述（可选）
func (b *ReActAgentBuilder) Description(desc string) *ReActAgentBuilder {
	b.description = desc
	return b
}

// Metadata 设置自定义元数据（可随 AgentState 持久化）
func (b *ReActAgentBuilder) Metadata(meta map[string]any) *ReActAgentBuilder {
	b.meta = meta
	return b
}

func (b *ReActAgentBuilder) SysPrompt(prompt string) *ReActAgentBuilder {
	b.sysPrompt = prompt
	return b
}

func (b *ReActAgentBuilder) Model(m model.ChatModel) *ReActAgentBuilder {
	b.chatModel = m
	return b
}

func (b *ReActAgentBuilder) Tools(tools ...tool.Tool) *ReActAgentBuilder {
	b.tools = append(b.tools, tools...)
	return b
}

// Toolkit 使用工具包（注册表/分组/执行器）；设置后与 Tools() 二选一优先使用 Toolkit
func (b *ReActAgentBuilder) Toolkit(tk *toolkit.Toolkit) *ReActAgentBuilder {
	b.toolkit = tk
	return b
}

func (b *ReActAgentBuilder) Memory(mem memory.Memory) *ReActAgentBuilder {
	b.memory = mem
	return b
}

func (b *ReActAgentBuilder) MaxIterations(n int) *ReActAgentBuilder {
	b.maxIterations = n
	return b
}

func (b *ReActAgentBuilder) Hooks(hooks ...hook.Hook) *ReActAgentBuilder {
	b.hooks = append(b.hooks, hooks...)
	return b
}

// HookManager 从管理器追加 Hook（Build 时会统一按优先级排序）
func (b *ReActAgentBuilder) HookManager(m *hook.Manager) *ReActAgentBuilder {
	if m == nil {
		return b
	}
	b.hooks = append(b.hooks, m.All()...)
	return b
}

// StreamHooks 注册流式/结构化事件 Hook（与经典 Hook 并存；有工具时本轮仍走 Chat 以保证 tool call 正确）
func (b *ReActAgentBuilder) StreamHooks(hooks ...hook.StreamHook) *ReActAgentBuilder {
	for _, h := range hooks {
		if h != nil {
			b.streamHooks = append(b.streamHooks, h)
		}
	}
	return b
}

func (b *ReActAgentBuilder) Build() (*ReActAgent, error) {
	if b.name == "" {
		return nil, errors.New("react agent: name is required")
	}
	if b.chatModel == nil {
		return nil, errors.New("react agent: model is required")
	}
	if b.memory == nil {
		b.memory = memory.NewInMemoryMemory()
	}

	toolMap := make(map[string]tool.Tool)
	if b.toolkit != nil {
		for _, t := range b.toolkit.Registry.List() {
			toolMap[t.Name()] = t
		}
	} else {
		for _, t := range b.tools {
			toolMap[t.Name()] = t
		}
	}

	a := &ReActAgent{
		Base: agent.NewBase(
			b.agentID,
			b.name,
			b.description,
			b.sysPrompt,
			cloneMeta(b.meta),
			b.hooks,
			b.streamHooks,
		),
		chatModel:     b.chatModel,
		tools:         b.tools,
		toolkit:       b.toolkit,
		memory:        b.memory,
		maxIterations: b.maxIterations,
		toolMap:       toolMap,
	}
	return a, nil
}

func cloneMeta(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (a *ReActAgent) Name() string { return a.Base.AgentName() }

// Shutdown gracefully closes the agent and waits for ongoing calls to finish.
func (a *ReActAgent) Shutdown(ctx context.Context) error {
	return a.Base.Shutdown(ctx)
}

// IsClosed reports whether the agent has been shut down.
func (a *ReActAgent) IsClosed() bool {
	return a.Base.IsClosed()
}

// TotalUsage returns the accumulated token usage across all calls.
func (a *ReActAgent) TotalUsage() model.ChatUsage {
	return a.Base.TotalUsage()
}

func (a *ReActAgent) addUsage(u model.ChatUsage) {
	a.Base.AddUsage(u)
}

func extractUsage(msg *message.Msg) model.ChatUsage {
	if msg == nil || len(msg.Metadata) == 0 {
		return model.ChatUsage{}
	}
	if v, ok := msg.Metadata["usage"]; ok {
		if u, ok := v.(model.ChatUsage); ok {
			return u
		}
	}
	return model.ChatUsage{}
}

// Call executes the ReAct loop and returns the final response
func (a *ReActAgent) Call(ctx context.Context, msg *message.Msg) (finalResponse *message.Msg, err error) {
	a.CallWg.Add(1)
	defer a.CallWg.Done()

	a.Mu.RLock()
	if a.Closed {
		a.Mu.RUnlock()
		return nil, ErrAgentClosed
	}
	a.Mu.RUnlock()

	// Build initial message history
	history, err := a.buildHistory(msg)
	if err != nil {
		_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name},
			Err:       err,
		})
		return nil, err
	}

	toolSpecs := a.toolSpecs()
	var chatOpts []model.ChatOption
	if len(toolSpecs) > 0 {
		chatOpts = append(chatOpts, model.WithTools(toolSpecs))
	}

	// PreCall event (turn-level)
	if _, _, err := a.fireStreamEvent(ctx, &hook.PreReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPreCall, Ts: time.Now(), Agent: a.Base.Name},
		Messages:  append([]*message.Msg(nil), history...),
		ModelName: a.chatModel.ModelName(),
	}); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name},
				Err:       err,
			})
		} else {
			_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventPostCall, Ts: time.Now(), Agent: a.Base.Name},
				Messages:  append([]*message.Msg(nil), history...),
				Response:  finalResponse,
			})
		}
		// PostCall classic hook
		_, _, _ = a.fireHooks(ctx, hook.HookPostCall, history, finalResponse, "", nil)
	}()

	calledTools := make(map[string]bool)
	for i := 0; i < a.maxIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		a.Mu.RLock()
		if a.Closed {
			a.Mu.RUnlock()
			return nil, ErrAgentClosed
		}
		a.Mu.RUnlock()

		// Fire before-model hooks（支持 InjectMessages 替换 history）
		var hr *hook.HookResult
		history, hr, err = a.fireHooks(ctx, hook.HookBeforeModel, history, nil, "", nil)
		if err != nil {
			return nil, err
		}
		if hr != nil && hr.Interrupt {
			finalResponse = hr.Override
			return finalResponse, nil
		}
		if hr != nil && hr.Override != nil {
			finalResponse = hr.Override
			break
		}

		// Call the model（有工具时 requestTools=true 走 Chat；无工具且注册了 StreamHook 时走 ChatStream）
		var response *message.Msg
		response, err = a.runModel(ctx, history, chatOpts, i, len(toolSpecs) > 0)
		if err != nil {
			if errors.Is(err, hook.ErrInterrupted) {
				return nil, err
			}
			return nil, err
		}
		a.addUsage(extractUsage(response))

		// Fire after-model hooks
		_, hr, err = a.fireHooks(ctx, hook.HookAfterModel, history, response, "", nil)
		if err != nil {
			return nil, err
		}
		if hr != nil && hr.StopAgent {
			finalResponse = hr.Override
			if finalResponse == nil {
				finalResponse = response
			}
			return finalResponse, nil
		}
		if hr != nil && hr.GotoReasoning {
			history = append(history, response)
			history = append(history, hr.GotoReasoningMsgs...)
			continue
		}
		if hr != nil && hr.Interrupt {
			finalResponse = hr.Override
			return finalResponse, nil
		}
		if hr != nil && hr.Override != nil {
			response = hr.Override
		}

		history = append(history, response)

		toolCalls := response.GetToolUseCalls()
		if len(toolCalls) == 0 {
			// No tool calls - this is the final answer
			_, hr, err = a.fireHooks(ctx, hook.HookBeforeFinish, history, response, "", nil)
			if err != nil {
				return nil, err
			}
			if hr != nil && hr.Override != nil {
				response = hr.Override
			}
			finalResponse = response
			break
		}

		// Execute tool calls
		toolResultMsg := message.NewMsg().Role(message.RoleTool)
		for _, tc := range toolCalls {
			// Fire before-tool hook
			_, hr, err = a.fireHooks(ctx, hook.HookBeforeTool, history, nil, tc.Name, tc.Input)
			if err != nil {
				return nil, err
			}
			if hr != nil && hr.Interrupt {
				finalResponse = hr.Override
				return finalResponse, nil
			}
			if _, _, err := a.fireStreamEvent(ctx, &hook.PreActingEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventPreActing, Ts: time.Now(), Agent: a.Base.Name, Iteration: i},
				Messages:  append([]*message.Msg(nil), history...),
				ToolName:  tc.Name,
				ToolInput: tc.Input,
			}); err != nil {
				if errors.Is(err, hook.ErrInterrupted) {
					return nil, err
				}
				return nil, err
			}

			tcStart := time.Now()
			result, toolErr := a.executeTool(ctx, tc.Name, tc.Input)
			tcElapsed := time.Since(tcStart).Seconds()

			var resultText string
			if toolErr != nil {
				resultText = fmt.Sprintf("error: %s", toolErr.Error())
			} else {
				resultJSON, _ := json.Marshal(result)
				resultText = string(resultJSON)
			}

			// 收集 ToolCallResult 用于 ToolMemory 总结
			if tcrCollector, ok := a.memory.(interface {
				AddToolCallResult(ctx context.Context, result memory.ToolCallResult) error
			}); ok {
				_ = tcrCollector.AddToolCallResult(ctx, memory.ToolCallResult{
					ToolName: tc.Name,
					Input:    tc.Input,
					Output:   resultText,
					Success:  toolErr == nil,
					TimeCost: tcElapsed,
				})
				calledTools[tc.Name] = true
			}

			singleResultMsg := message.NewMsg().Role(message.RoleTool).Content(
				message.NewToolResultBlock(tc.ID, []message.ContentBlock{
					message.NewTextBlock(resultText),
				}, toolErr != nil),
			).Build()

			if _, _, err := a.fireStreamEvent(ctx, &hook.PostActingEvent{
				BaseEvent: hook.BaseEvent{Type: hook.EventPostActing, Ts: time.Now(), Agent: a.Base.Name, Iteration: i},
				Messages:  append([]*message.Msg(nil), history...),
				ToolName:  tc.Name,
				ToolInput: tc.Input,
				Result:    result,
				Err:       toolErr,
				ResultMsg: singleResultMsg,
			}); err != nil {
				if errors.Is(err, hook.ErrInterrupted) {
					return nil, err
				}
				return nil, err
			}

			// Fire after-tool hook
			_, afterHr, afterErr := a.fireHooks(ctx, hook.HookAfterTool, history, nil, tc.Name, tc.Input)
			if afterErr != nil {
				err = afterErr
				return nil, err
			}
			if afterHr != nil && afterHr.StopAgent {
				finalResponse = afterHr.Override
				if finalResponse == nil {
					finalResponse = singleResultMsg
				}
				return finalResponse, nil
			}
			if afterHr != nil && afterHr.Interrupt {
				finalResponse = afterHr.Override
				return finalResponse, nil
			}

			toolResultMsg.Content(singleResultMsg.Content...)
		}
		history = append(history, toolResultMsg.Build())
	}

	if finalResponse == nil {
		err = errors.New("react agent: max iterations reached without final answer")
		return nil, err
	}

	// 批量总结工具调用
	if tcrCollector, ok := a.memory.(interface {
		SummarizeToolUsage(ctx context.Context, toolName string) error
	}); ok {
		for toolName := range calledTools {
			_ = tcrCollector.SummarizeToolUsage(ctx, toolName)
		}
	}

	// Persist to memory
	_ = a.memory.Add(msg)
	_ = a.memory.Add(finalResponse)

	return finalResponse, nil
}

// CallStream executes the ReAct loop with streaming output
func (a *ReActAgent) CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error) {
	ch := make(chan *message.Msg, 16)
	go func() {
		defer close(ch)
		resp, err := a.Call(ctx, msg)
		if err != nil {
			// Send error message
			ch <- message.NewMsg().
				Role(message.RoleAssistant).
				TextContent(fmt.Sprintf("error: %s", err.Error())).
				Build()
			return
		}
		ch <- resp
	}()
	return ch, nil
}

// buildHistory assembles system prompt + memory + new user message
func (a *ReActAgent) buildHistory(userMsg *message.Msg) ([]*message.Msg, error) {
	var history []*message.Msg

	if a.Base.SysPrompt != "" {
		history = append(history, message.NewMsg().
			Role(message.RoleSystem).
			TextContent(a.Base.SysPrompt).
			Build())
	}

	var memMsgs []*message.Msg
	var err error
	if pm, ok := a.memory.(interface {
		GetMemoryForPrompt(prepend bool) ([]*message.Msg, error)
	}); ok {
		memMsgs, err = pm.GetMemoryForPrompt(true)
	} else {
		memMsgs, err = a.memory.GetAll()
	}
	if err != nil {
		return nil, err
	}
	history = append(history, memMsgs...)
	history = append(history, userMsg)
	return history, nil
}

// toolSpecs converts tools to model.ToolSpec slice
func (a *ReActAgent) toolSpecs() []model.ToolSpec {
	if a.toolkit != nil {
		return a.toolkit.ActiveToolSpecs()
	}
	specs := make([]model.ToolSpec, 0, len(a.tools))
	for _, t := range a.tools {
		specs = append(specs, t.Spec())
	}
	return specs
}

// executeTool finds and runs the named tool
func (a *ReActAgent) executeTool(ctx context.Context, name string, input map[string]any) (any, error) {
	if a.toolkit != nil {
		return a.toolkit.ExecuteTool(ctx, name, input)
	}
	t, ok := a.toolMap[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return t.Execute(ctx, input)
}

// fireHooks fires all registered hooks for the given point；支持 InjectMessages 链式更新 Messages
func (a *ReActAgent) fireHooks(
	ctx context.Context,
	point hook.HookPoint,
	messages []*message.Msg,
	response *message.Msg,
	toolName string,
	toolInput map[string]any,
) ([]*message.Msg, *hook.HookResult, error) {
	return a.Base.FireHooks(ctx, point, messages, response, toolName, toolInput)
}

// Ensure ReActAgent satisfies agent.Agent (compile-time check via blank import in agent package)
var _ interface {
	Name() string
	Call(ctx context.Context, msg *message.Msg) (*message.Msg, error)
	CallStream(ctx context.Context, msg *message.Msg) (<-chan *message.Msg, error)
} = (*ReActAgent)(nil)
