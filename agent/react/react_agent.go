package react

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/interruption"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
	"github.com/linkerlin/agentscope.go/runcontext"
	"github.com/linkerlin/agentscope.go/shutdown"
	"github.com/linkerlin/agentscope.go/state"
	"github.com/linkerlin/agentscope.go/tool"
	"github.com/linkerlin/agentscope.go/tool/file"
	"github.com/linkerlin/agentscope.go/tool/shell"
	tasktool "github.com/linkerlin/agentscope.go/tool/task"
	"github.com/linkerlin/agentscope.go/toolkit"
	"github.com/linkerlin/agentscope.go/workspace"
)

const defaultMaxIterations = 10

// ErrAgentClosed is returned when calling a shut-down agent.
var ErrAgentClosed = errors.New("react agent: agent is closed")

// hookInterruptError carries an override from a hook that interrupted during a concurrent batch.
type hookInterruptError struct {
	override *message.Msg
}

func (e *hookInterruptError) Error() string { return "hook interrupted" }

// ReActAgent implements the ReAct (Reasoning + Acting) pattern
type ReActAgent struct {
	*agent.Base

	chatModel      model.ChatModel
	tools          []tool.Tool
	toolkit        *toolkit.Toolkit
	memory         memory.Memory
	maxIterations  int
	toolMap        map[string]tool.Tool
	shutdownConfig shutdown.GracefulShutdownConfig

	// V2 runtime state (suspend-resume support)
	runtimeMu    sync.Mutex
	runtimeState *agent.AgentState
	waiters      map[string]chan event.AgentEvent // confirm_id -> waiter channel
	waitersMu    sync.Mutex

	// V2 production capabilities
	permissionEngine *permission.Engine
	workspace        workspace.Workspace
	eventBus         *event.Bus
	taskStore        *state.TaskStore

	// Context compression (PyV2 compress_context)
	contextConfig agent.ContextConfig
	contextSize   int
	offloader     workspace.Offloader
}

// ReActAgentBuilder provides a fluent API for constructing ReActAgent
type ReActAgentBuilder struct {
	agentID        string
	name           string
	description    string
	sysPrompt      string
	chatModel      model.ChatModel
	tools          []tool.Tool
	toolkit        *toolkit.Toolkit
	memory         memory.Memory
	maxIterations  int
	hooks          []hook.Hook
	streamHooks    []hook.StreamHook
	middlewares    []middleware.Middleware
	meta           map[string]any
	shutdownConfig shutdown.GracefulShutdownConfig

	// V2 fields
	permissionEngine *permission.Engine
	workspace        workspace.Workspace
	eventBus         *event.Bus
	taskStore        *state.TaskStore

	contextConfig agent.ContextConfig
	contextSize   int
	offloader     workspace.Offloader
}

// Builder returns a new ReActAgentBuilder
func Builder() *ReActAgentBuilder {
	return &ReActAgentBuilder{
		maxIterations: defaultMaxIterations,
	}
}

//nolint:revive
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

//nolint:revive
func (b *ReActAgentBuilder) SysPrompt(prompt string) *ReActAgentBuilder {
	b.sysPrompt = prompt
	return b
}

//nolint:revive
func (b *ReActAgentBuilder) Model(m model.ChatModel) *ReActAgentBuilder {
	b.chatModel = m
	return b
}

//nolint:revive
func (b *ReActAgentBuilder) Tools(tools ...tool.Tool) *ReActAgentBuilder {
	b.tools = append(b.tools, tools...)
	return b
}

// Toolkit 使用工具包（注册表/分组/执行器）；设置后与 Tools() 二选一优先使用 Toolkit
func (b *ReActAgentBuilder) Toolkit(tk *toolkit.Toolkit) *ReActAgentBuilder {
	b.toolkit = tk
	return b
}

//nolint:revive
func (b *ReActAgentBuilder) Memory(mem memory.Memory) *ReActAgentBuilder {
	b.memory = mem
	return b
}

//nolint:revive
func (b *ReActAgentBuilder) MaxIterations(n int) *ReActAgentBuilder {
	b.maxIterations = n
	return b
}

// ShutdownConfig sets the graceful-shutdown configuration for the agent.
func (b *ReActAgentBuilder) ShutdownConfig(cfg shutdown.GracefulShutdownConfig) *ReActAgentBuilder {
	b.shutdownConfig = cfg
	return b
}

//nolint:revive
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

// Middlewares registers agent-level lifecycle middleware (on_reply / on_reasoning / on_acting / on_model_call / on_system_prompt).
func (b *ReActAgentBuilder) Middlewares(mws ...middleware.Middleware) *ReActAgentBuilder {
	for _, mw := range mws {
		if mw != nil {
			b.middlewares = append(b.middlewares, mw)
		}
	}
	return b
}

// PermissionEngine sets the V2 permission engine for HITL tool confirmation.
func (b *ReActAgentBuilder) PermissionEngine(pe *permission.Engine) *ReActAgentBuilder {
	b.permissionEngine = pe
	return b
}

// Workspace sets the V2 workspace abstraction for sandboxed tool execution.
func (b *ReActAgentBuilder) Workspace(ws workspace.Workspace) *ReActAgentBuilder {
	b.workspace = ws
	return b
}

// WithEventBus attaches an event bus for broadcasting AgentEvents to external
// consumers (e.g. Studio UI, loggers).
func (b *ReActAgentBuilder) WithEventBus(bus *event.Bus) *ReActAgentBuilder {
	b.eventBus = bus
	return b
}

// ContextConfig sets automatic context compression parameters (PyV2 ContextConfig).
func (b *ReActAgentBuilder) ContextConfig(cfg agent.ContextConfig) *ReActAgentBuilder {
	b.contextConfig = cfg
	return b
}

// ContextSize sets the model context window size used for compression thresholds.
// When zero, ResolveContextSize falls back to the model or library default.
func (b *ReActAgentBuilder) ContextSize(n int) *ReActAgentBuilder {
	b.contextSize = n
	return b
}

// Offloader sets the workspace offloader used when tool results are truncated.
func (b *ReActAgentBuilder) Offloader(o workspace.Offloader) *ReActAgentBuilder {
	b.offloader = o
	return b
}

// WithTaskStore attaches a task store and registers TaskCreate/Get/List/Update tools.
func (b *ReActAgentBuilder) WithTaskStore(store *state.TaskStore) *ReActAgentBuilder {
	if store == nil {
		store = state.NewTaskStore()
	}
	b.taskStore = store
	b.tools = append(b.tools, tasktool.RegisterTools(store)...)
	return b
}

// TaskStore returns the agent's task store (may be nil).
func (a *ReActAgent) TaskStore() *state.TaskStore { return a.taskStore }

//nolint:revive
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

	if b.permissionEngine != nil {
		b.permissionEngine.SetToolResolver(func(name string) tool.Tool {
			return toolMap[name]
		})
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
			b.middlewares...,
		),
		chatModel:        b.chatModel,
		tools:            b.tools,
		toolkit:          b.toolkit,
		memory:           b.memory,
		maxIterations:    b.maxIterations,
		toolMap:          toolMap,
		shutdownConfig:   b.shutdownConfig,
		waiters:          make(map[string]chan event.AgentEvent),
		permissionEngine: b.permissionEngine,
		workspace:        b.workspace,
		eventBus:         b.eventBus,
		taskStore:        b.taskStore,
		contextConfig:    b.contextConfig,
		contextSize:      b.contextSize,
		offloader:        b.offloader,
	}
	if a.contextConfig.TriggerRatio <= 0 {
		a.contextConfig = agent.DefaultContextConfig()
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
// Call executes the agent synchronously (V1 API).
func (a *ReActAgent) Call(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	return a.Base.Call(ctx, msg, a.replyInternal)
}

// Reply consumes the full event stream and returns the final assembled
// assistant message. It is the synchronous counterpart to ReplyStream,
// aligned with Python v2's reply() method.
func (a *ReActAgent) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	ch, err := a.ReplyStream(ctx, msg)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for ev := range ch {
		if e, ok := ev.(*event.ErrorEvent); ok && e.Err != "" {
			lastErr = errors.New(e.Err)
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}

	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState == nil || len(a.runtimeState.Messages) == 0 {
		return nil, errors.New("react agent: reply completed but no message produced")
	}
	for i := len(a.runtimeState.Messages) - 1; i >= 0; i-- {
		if a.runtimeState.Messages[i].Role == message.RoleAssistant {
			return a.runtimeState.Messages[i], nil
		}
	}
	return nil, errors.New("react agent: reply completed but no assistant message found")
}

// replyInternal is the core ReAct logic executed inside Base.Call lifecycle.
func (a *ReActAgent) replyInternal(ctx context.Context, msg *message.Msg) (finalResponse *message.Msg, err error) {
	a.CallWg.Add(1)
	defer a.CallWg.Done()

	a.Mu.RLock()
	if a.Closed {
		a.Mu.RUnlock()
		return nil, ErrAgentClosed
	}
	a.Mu.RUnlock()

	// Reset interrupt flag at the beginning of each call (align with Java acquireExecution)
	a.ResetInterrupt()

	// Fire PreCall classic hook
	preCallMsgs, hr, err := a.fireHooks(ctx, hook.HookPreCall, []*message.Msg{msg}, nil, "", nil)
	if err != nil {
		return nil, err
	}
	if hr != nil && (hr.Interrupt || hr.Override != nil) {
		return hr.Override, nil
	}
	inputMsg := msg
	if len(preCallMsgs) > 0 {
		inputMsg = preCallMsgs[0]
	}

	// Build initial message history
	history, err := a.buildHistory(ctx, inputMsg)
	if err != nil {
		_, _, _ = a.fireStreamEvent(ctx, &hook.ErrorEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventError, Ts: time.Now(), Agent: a.Base.Name},
			Err:       err,
		})
		return nil, err
	}

	toolSpecs := a.toolSpecs(ctx)
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

		if err := a.CheckInterrupted(); err != nil {
			return a.handleInterrupt(ctx, msg, history, nil)
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

		if err := a.CompressContext(ctx, inputMsg, toolSpecs); err != nil {
			return nil, err
		}
		history, err = a.syncHistoryWithMemory(ctx, inputMsg, history)
		if err != nil {
			return nil, err
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

		if err := a.CheckInterrupted(); err != nil {
			return a.handleInterrupt(ctx, msg, history, response.GetToolUseCalls())
		}

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

		// Execute tool calls concurrently
		if err := a.CheckInterrupted(); err != nil {
			return a.handleInterrupt(ctx, msg, history, toolCalls)
		}

		replyID := ""
		a.runtimeMu.Lock()
		if a.runtimeState != nil {
			replyID = a.runtimeState.ReplyID
		}
		a.runtimeMu.Unlock()

		externalResults, err := a.handleExternalToolCalls(ctx, replyID, toolCalls)
		if err != nil {
			return nil, err
		}

		type toolRunResult struct {
			contentBlocks   []message.ContentBlock
			singleResultMsg *message.Msg
			afterHr         *hook.HookResult
			elapsed         float64
			toolName        string
			toolInput       map[string]any
			hasTcr          bool
			tcr             memory.ToolCallResult
		}

		results := make([]toolRunResult, len(toolCalls))
		var g errgroup.Group

		for idx, tc := range toolCalls {
			tc := tc
			idx := idx
			if ext, ok := externalResults[idx]; ok {
				blocks := a.compressToolResultBlocks(ctx, tc.ID, ext.blocks, ext.isErr)
				singleResultMsg := message.NewMsg().Role(message.RoleTool).Content(
					message.NewToolResultBlock(tc.ID, blocks, ext.isErr),
				).Build()
				results[idx] = toolRunResult{
					contentBlocks:   blocks,
					singleResultMsg: singleResultMsg,
					toolName:        tc.Name,
					toolInput:       tc.Input,
				}
				continue
			}
			g.Go(func() error {
				// Fire before-tool hook
				_, hr, err := a.fireHooks(ctx, hook.HookBeforeTool, history, nil, tc.Name, tc.Input)
				if err != nil {
					return err
				}
				if hr != nil && hr.Interrupt {
					return &hookInterruptError{override: hr.Override}
				}

				if _, _, err := a.fireStreamEvent(ctx, &hook.PreActingEvent{
					BaseEvent: hook.BaseEvent{Type: hook.EventPreActing, Ts: time.Now(), Agent: a.Base.Name, Iteration: i},
					Messages:  append([]*message.Msg(nil), history...),
					ToolName:  tc.Name,
					ToolInput: tc.Input,
				}); err != nil {
					if errors.Is(err, hook.ErrInterrupted) {
						return err
					}
					return err
				}

				tcStart := time.Now()
				resp, toolErr := a.executeTool(ctx, tc.Name, tc.Input)
				tcElapsed := time.Since(tcStart).Seconds()

				var contentBlocks []message.ContentBlock
				if toolErr != nil {
					contentBlocks = []message.ContentBlock{message.NewTextBlock(fmt.Sprintf("error: %s", toolErr.Error()))}
				} else if resp != nil && len(resp.Content) > 0 {
					contentBlocks = resp.Content
				} else {
					contentBlocks = []message.ContentBlock{message.NewTextBlock("")}
				}
				contentBlocks = a.compressToolResultBlocks(ctx, tc.ID, contentBlocks, toolErr != nil)

				singleResultMsg := message.NewMsg().Role(message.RoleTool).Content(
					message.NewToolResultBlock(tc.ID, contentBlocks, toolErr != nil),
				).Build()

				if _, _, err := a.fireStreamEvent(ctx, &hook.PostActingEvent{
					BaseEvent: hook.BaseEvent{Type: hook.EventPostActing, Ts: time.Now(), Agent: a.Base.Name, Iteration: i},
					Messages:  append([]*message.Msg(nil), history...),
					ToolName:  tc.Name,
					ToolInput: tc.Input,
					Result:    resp,
					Err:       toolErr,
					ResultMsg: singleResultMsg,
				}); err != nil {
					if errors.Is(err, hook.ErrInterrupted) {
						return err
					}
					return err
				}

				// Fire after-tool hook
				_, afterHr, afterErr := a.fireHooks(ctx, hook.HookAfterTool, history, nil, tc.Name, tc.Input)
				if afterErr != nil {
					return afterErr
				}

				var hasTcr bool
				var tcr memory.ToolCallResult
				if _, ok := a.memory.(interface {
					AddToolCallResult(ctx context.Context, result memory.ToolCallResult) error
				}); ok {
					outputText := ""
					for _, b := range contentBlocks {
						if tb, ok := b.(*message.TextBlock); ok {
							outputText += tb.Text
						}
					}
					tcr = memory.ToolCallResult{
						ToolName: tc.Name,
						Input:    tc.Input,
						Output:   outputText,
						Success:  toolErr == nil,
						TimeCost: tcElapsed,
					}
					hasTcr = true
				}

				results[idx] = toolRunResult{
					contentBlocks:   contentBlocks,
					singleResultMsg: singleResultMsg,
					afterHr:         afterHr,
					elapsed:         tcElapsed,
					toolName:        tc.Name,
					toolInput:       tc.Input,
					hasTcr:          hasTcr,
					tcr:             tcr,
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			if hi, ok := err.(*hookInterruptError); ok {
				return hi.override, nil
			}
			return nil, err
		}

		toolResultMsg := message.NewMsg().Role(message.RoleTool)
		for _, r := range results {
			if r.afterHr != nil && r.afterHr.StopAgent {
				finalResponse = r.afterHr.Override
				if finalResponse == nil {
					finalResponse = r.singleResultMsg
				}
				return finalResponse, nil
			}
			if r.afterHr != nil && r.afterHr.Interrupt {
				finalResponse = r.afterHr.Override
				return finalResponse, nil
			}

			if r.hasTcr {
				if tcrCollector, ok := a.memory.(interface {
					AddToolCallResult(ctx context.Context, result memory.ToolCallResult) error
				}); ok {
					_ = tcrCollector.AddToolCallResult(ctx, r.tcr)
					calledTools[r.toolName] = true
				}
			}

			toolResultMsg.Content(r.singleResultMsg.Content...)
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

// handleInterrupt processes an interruption that occurred during ReAct execution.
// It mirrors Java ReActAgent.handleInterrupt behaviour:
//   - SYSTEM source -> apply PartialReasoningPolicy, return shutdown error
//   - USER source   -> generate a recovery message, persist to memory, return it
//
//nolint:unparam
func (a *ReActAgent) handleInterrupt(ctx context.Context, originalMsg *message.Msg, history []*message.Msg, pending []*message.ToolUseBlock) (*message.Msg, error) {
	ic := a.CreateInterruptContext(pending)

	if ic.Source == interruption.SourceSystem {
		// Apply partial-reasoning policy
		if a.shutdownConfig.PartialReasoningPolicy == shutdown.Save && len(history) > 0 {
			// Persist the last assistant turn (if any) so the agent can resume later.
			for _, m := range history {
				if m.Role == message.RoleAssistant {
					_ = a.memory.Add(m)
				}
			}
		}
		return nil, fmt.Errorf("%w: source=%s", ErrAgentClosed, ic.Source)
	}

	recoveryText := "I noticed that you have interrupted me. What can I do for you?"
	recoveryMsg := message.NewMsg().
		Role(message.RoleAssistant).
		Name(a.Name()).
		TextContent(recoveryText).
		Build()

	_ = a.memory.Add(recoveryMsg)
	return recoveryMsg, nil
}

// Observe receives a message without generating a reply (aligns with Python AgentBase).
func (a *ReActAgent) Observe(ctx context.Context, msg *message.Msg) error {
	return a.Base.Observe(ctx, msg, a.observeInternal)
}

func (a *ReActAgent) observeInternal(ctx context.Context, msg *message.Msg) error {
	if msg == nil {
		return nil
	}
	return a.memory.Add(msg)
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

// buildHistory assembles system prompt + memory + new user message.
// If the memory implements ReMeMemory, PreReasoningPrepare is applied automatically.
func (a *ReActAgent) buildHistory(ctx context.Context, userMsg *message.Msg) ([]*message.Msg, error) {
	var history []*message.Msg

	if a.Base.SysPrompt != "" {
		prompt := a.Base.SysPrompt
		if chain := a.Base.MiddlewareChain(); chain != nil {
			var err error
			prompt, err = middleware.ApplySystemPrompt(ctx, a.Base, chain, prompt)
			if err != nil {
				return nil, err
			}
		}
		history = append(history, message.NewMsg().
			Role(message.RoleSystem).
			TextContent(prompt).
			Build())
	}

	if summary := a.getCompressedSummary(); summary != "" && !a.hasPreReasoningMemory() {
		history = append(history, message.NewMsg().
			Role(message.RoleUser).
			TextContent(summary).
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

	// Auto-integrate ReMe memory compression
	if rm, ok := a.memory.(interface {
		PreReasoningPrepare(ctx context.Context, history []*message.Msg) ([]*message.Msg, *memory.CompactSummary, error)
	}); ok {
		prepared, _, err := rm.PreReasoningPrepare(ctx, history)
		if err != nil {
			return nil, err
		}
		history = prepared
	}

	return history, nil
}

// toolSpecs converts tools to model.ToolSpec slice, including session tools from ctx.
func (a *ReActAgent) toolSpecs(ctx context.Context) []model.ToolSpec {
	var specs []model.ToolSpec
	if a.toolkit != nil {
		specs = append(specs, a.toolkit.ActiveToolSpecs()...)
	} else {
		for _, t := range a.tools {
			specs = append(specs, t.Spec())
		}
	}
	for _, t := range sessionToolsFromContext(ctx) {
		specs = append(specs, t.Spec())
	}
	return specs
}

// executeTool finds and runs the named tool. If a workspace is configured,
// it attempts to bind the workspace to the tool before execution.
func (a *ReActAgent) executeTool(ctx context.Context, name string, input map[string]any) (*tool.Response, error) {
	final := func(ctx context.Context) (*tool.Response, error) {
		return a.actingImpl(ctx, name, input)
	}
	chain := a.Base.MiddlewareChain()
	if chain != nil && len(chain.Acting) > 0 {
		actingInput := &middleware.ActingInput{ToolName: name, ToolInput: input}
		handler := middleware.ChainActing(chain, a.Base, actingInput, final)
		return handler(ctx)
	}
	return final(ctx)
}

func (a *ReActAgent) actingImpl(ctx context.Context, name string, input map[string]any) (*tool.Response, error) {
	if resp, ok, err := a.executeSessionTool(ctx, name, input); ok {
		return resp, err
	}
	if a.toolkit != nil {
		if a.workspace != nil {
			if t, ok := a.toolkit.Registry.Get(name); ok {
				bindWorkspaceToTool(t, a.workspace)
			}
		}
		return a.toolkit.ExecuteTool(ctx, name, input)
	}
	t, ok := a.toolMap[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	if a.workspace != nil {
		bindWorkspaceToTool(t, a.workspace)
	}
	return t.Execute(ctx, input)
}

func (a *ReActAgent) isExternalTool(ctx context.Context, name string) bool {
	t, ok := a.lookupTool(ctx, name)
	if !ok {
		return false
	}
	if ext, ok := t.(tool.ExternalChecker); ok {
		return ext.IsExternalTool()
	}
	return false
}

func (a *ReActAgent) lookupTool(ctx context.Context, name string) (tool.Tool, bool) {
	for _, t := range sessionToolsFromContext(ctx) {
		if t != nil && t.Name() == name {
			return t, true
		}
	}
	if a.toolkit != nil {
		return a.toolkit.Registry.Get(name)
	}
	t, ok := a.toolMap[name]
	return t, ok
}

func (a *ReActAgent) executeSessionTool(ctx context.Context, name string, input map[string]any) (*tool.Response, bool, error) {
	for _, t := range sessionToolsFromContext(ctx) {
		if t == nil || t.Name() != name {
			continue
		}
		if a.workspace != nil {
			bindWorkspaceToTool(t, a.workspace)
		}
		resp, err := t.Execute(ctx, input)
		return resp, true, err
	}
	return nil, false, nil
}

func sessionToolsFromContext(ctx context.Context) []tool.Tool {
	return runcontext.Tools(ctx)
}

// bindWorkspaceToTool uses reflection-free type assertions to bind a workspace
// to tools that expose WithWorkspace methods.
func bindWorkspaceToTool(t tool.Tool, ws workspace.Workspace) {
	switch wt := t.(type) {
	case *file.ReadFileTool:
		wt.WithWorkspace(ws)
	case *file.WriteFileTool:
		wt.WithWorkspace(ws)
	case *file.InsertTextFileTool:
		wt.WithWorkspace(ws)
	case *file.ListDirectoryTool:
		wt.WithWorkspace(ws)
	case *file.EditFileTool:
		wt.WithWorkspace(ws)
	case *file.GlobTool:
		wt.WithWorkspace(ws)
	case *file.GrepTool:
		wt.WithWorkspace(ws)
	case *shell.ShellCommandTool:
		wt.WithWorkspace(ws)
	}
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

// InjectEvent allows an external consumer to inject a resume event into a
// suspended agent. Supported events: UserConfirmResultEvent, ExternalExecutionResultEvent.
func (a *ReActAgent) InjectEvent(ctx context.Context, ev event.AgentEvent) error {
	switch e := ev.(type) {
	case *event.UserConfirmResultEvent:
		return a.signalWaiter(e.ConfirmID, ev)
	case *event.ExternalExecutionResultEvent:
		return a.signalWaiter(e.ConfirmID, ev)
	default:
		return fmt.Errorf("react agent: unsupported inject event type %T", ev)
	}
}

// signalWaiter delivers an event to a registered waiter channel.
func (a *ReActAgent) signalWaiter(confirmID string, ev event.AgentEvent) error {
	a.waitersMu.Lock()
	ch, ok := a.waiters[confirmID]
	a.waitersMu.Unlock()
	if !ok {
		return fmt.Errorf("react agent: no waiter for confirm_id %s", confirmID)
	}
	select {
	case ch <- ev:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("react agent: timeout signalling waiter for confirm_id %s", confirmID)
	}
}

// waitForExternalEvent blocks until an external event is injected for the given confirmID.
func (a *ReActAgent) waitForExternalEvent(ctx context.Context, confirmID string) (event.AgentEvent, error) {
	ch := make(chan event.AgentEvent, 1)
	a.waitersMu.Lock()
	a.waiters[confirmID] = ch
	a.waitersMu.Unlock()

	defer func() {
		a.waitersMu.Lock()
		delete(a.waiters, confirmID)
		a.waitersMu.Unlock()
	}()

	select {
	case ev := <-ch:
		return ev, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SaveState captures the current runtime state.
func (a *ReActAgent) SaveState() (*agent.AgentState, error) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.runtimeState == nil {
		return nil, errors.New("react agent: no active runtime state")
	}
	// Deep copy messages to avoid races
	st := *a.runtimeState
	if len(a.runtimeState.Messages) > 0 {
		st.Messages = make([]*message.Msg, len(a.runtimeState.Messages))
		copy(st.Messages, a.runtimeState.Messages)
	}
	st.UpdatedAt = time.Now()
	return &st, nil
}

// LoadState restores runtime state. Note: ChatModel, tools, and memory
// must still be injected by the caller after LoadState.
func (a *ReActAgent) LoadState(st *agent.AgentState) error {
	if st == nil {
		return errors.New("react agent: nil state")
	}
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.runtimeState = st
	return nil
}

// Ensure ReActAgent satisfies agent.Agent (compile-time check)
var _ agent.Agent = (*ReActAgent)(nil)
