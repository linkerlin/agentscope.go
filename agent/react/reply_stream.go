package react

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/permission"
)

// ReplyStream implements the V2 true event-stream API.
// It runs the ReAct loop in a goroutine and emits fine-grained events on the returned channel.
func (a *ReActAgent) ReplyStream(ctx context.Context, msg *message.Msg) (<-chan event.AgentEvent, error) {
	a.Mu.RLock()
	if a.Closed {
		a.Mu.RUnlock()
		return nil, ErrAgentClosed
	}
	a.Mu.RUnlock()

	out := make(chan event.AgentEvent, 64)

	if a.eventBus != nil {
		// Bridge: copy every event to both the returned channel and the event bus.
		bridge := make(chan event.AgentEvent, 64)
		go func() {
			defer close(out)
			for ev := range bridge {
				out <- ev
				a.eventBus.Publish(ev)
			}
		}()
		go a.replyStreamLoop(ctx, msg, bridge)
		return out, nil
	}

	go a.replyStreamLoop(ctx, msg, out)
	return out, nil
}

// replyStreamLoop is the core event-driven ReAct logic.
func (a *ReActAgent) replyStreamLoop(ctx context.Context, msg *message.Msg, out chan<- event.AgentEvent) {
	defer close(out)

	a.runtimeMu.Lock()
	var replyID string
	var isResume bool
	if a.runtimeState != nil && a.runtimeState.SuspendedAt != nil {
		replyID = a.runtimeState.ReplyID
		isResume = true
	} else {
		replyID = uuid.New().String()
		a.runtimeState = &agent.AgentState{
			Version:   "v2",
			ReplyID:   replyID,
			CurIter:   0,
			MaxIters:  a.maxIterations,
			AgentName: a.Name(),
			AgentID:   a.Base.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
	a.runtimeMu.Unlock()

	out <- event.NewReplyStart(replyID, a.Name())
	defer func() {
		out <- event.NewReplyEnd(replyID, a.Name())
	}()

	var err error
	if isResume {
		_, err = a.resumeReplyStreamInternal(ctx, msg, out, replyID)
	} else {
		// For backward compatibility with Base.Call lifecycle, we keep the wrapper
		// but the internal loop emits events directly.
		_, err = a.Base.Call(ctx, msg, func(innerCtx context.Context, input *message.Msg) (*message.Msg, error) {
			return a.replyStreamInternal(innerCtx, input, out, replyID)
		})
	}
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			out <- event.NewError(replyID, err)
		}
	}
}

// replyStreamInternal executes the ReAct loop in event-stream mode.
func (a *ReActAgent) replyStreamInternal(
	ctx context.Context,
	msg *message.Msg,
	out chan<- event.AgentEvent,
	replyID string,
) (*message.Msg, error) {
	a.CallWg.Add(1)
	defer a.CallWg.Done()

	a.ResetInterrupt()

	// PreCall classic hook
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

	// Build history
	history, err := a.buildHistory(ctx, inputMsg)
	if err != nil {
		out <- event.NewError(replyID, err)
		return nil, err
	}

	toolSpecs := a.toolSpecs(ctx)
	var chatOpts []model.ChatOption
	if len(toolSpecs) > 0 {
		chatOpts = append(chatOpts, model.WithTools(toolSpecs))
	}

	// PreCall stream event (no-op for now; pre-reasoning events are emitted inside runModelStream)

	var finalResponse *message.Msg
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

		// Update runtime state
		a.runtimeMu.Lock()
		if a.runtimeState != nil {
			a.runtimeState.CurIter = i
			a.runtimeState.Messages = append([]*message.Msg(nil), history...)
			a.runtimeState.UpdatedAt = time.Now()
		}
		a.runtimeMu.Unlock()

		// Before-model hooks
		history, hr, err = a.fireHooks(ctx, hook.HookBeforeModel, history, nil, "", nil)
		if err != nil {
			return nil, err
		}
		if hr != nil && hr.Interrupt {
			return hr.Override, nil
		}
		if hr != nil && hr.Override != nil {
			finalResponse = hr.Override
			break
		}

		if err := a.CompressContext(ctx, inputMsg, toolSpecs); err != nil {
			out <- event.NewError(replyID, err)
			return nil, err
		}
		history, err = a.syncHistoryWithMemory(ctx, inputMsg, history)
		if err != nil {
			out <- event.NewError(replyID, err)
			return nil, err
		}

		// Run model in streaming mode, emitting events directly
		response, err := a.runModelStream(ctx, history, chatOpts, i, len(toolSpecs) > 0, out, replyID)
		if err != nil {
			if errors.Is(err, hook.ErrInterrupted) {
				return nil, err
			}
			out <- event.NewError(replyID, err)
			return nil, err
		}
		a.addUsage(extractUsage(response))

		if err := a.CheckInterrupted(); err != nil {
			return a.handleInterrupt(ctx, msg, history, response.GetToolUseCalls())
		}

		// After-model hooks
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

		// Update runtime state after model response so that SaveState captures
		// the assistant message (including any tool calls) for reconnect resume.
		a.runtimeMu.Lock()
		if a.runtimeState != nil {
			a.runtimeState.Messages = append([]*message.Msg(nil), history...)
			a.runtimeState.UpdatedAt = time.Now()
		}
		a.runtimeMu.Unlock()

		toolCalls := response.GetToolUseCalls()
		if len(toolCalls) == 0 {
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

		// Execute tools concurrently, emitting events
		if err := a.CheckInterrupted(); err != nil {
			return a.handleInterrupt(ctx, msg, history, toolCalls)
		}

		toolResultMsg, err := a.executeToolsStream(ctx, history, toolCalls, out, replyID, i)
		if err != nil {
			return nil, err
		}
		history = append(history, toolResultMsg)
	}

	if finalResponse == nil {
		out <- event.NewExceedMaxIters(replyID, a.maxIterations)
		err := errors.New("react agent: max iterations reached without final answer")
		return nil, err
	}

	// Persist to memory
	_ = a.memory.Add(msg)
	_ = a.memory.Add(finalResponse)

	return finalResponse, nil
}

// resumeReplyStreamInternal resumes a reply that was previously suspended at a
// RequireUserConfirmEvent. It restores the saved history, waits for the
// UserConfirmResultEvent, executes the confirmed tools, and performs one final
// reasoning round. This is a minimal viable reconnect-resume path; full
// multi-iteration resume can be added later.
func (a *ReActAgent) resumeReplyStreamInternal(
	ctx context.Context,
	msg *message.Msg,
	out chan<- event.AgentEvent,
	replyID string,
) (*message.Msg, error) {
	a.CallWg.Add(1)
	defer a.CallWg.Done()

	a.ResetInterrupt()

	// Restore history and suspend metadata from runtimeState.
	a.runtimeMu.Lock()
	history := append([]*message.Msg(nil), a.runtimeState.Messages...)
	confirmID := a.runtimeState.WaitConfirmID
	startIter := a.runtimeState.CurIter
	a.runtimeMu.Unlock()

	// Find the last assistant message that contains tool calls.
	var toolCalls []*message.ToolUseBlock
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == message.RoleAssistant {
			toolCalls = history[i].GetToolUseCalls()
			break
		}
	}
	if len(toolCalls) == 0 {
		return nil, errors.New("react agent resume: no tool calls in saved history")
	}

	// Re-emit the suspend event so the reconnecting client knows we are waiting.
	var calls []event.ToolCallSummary
	for _, tc := range toolCalls {
		calls = append(calls, event.ToolCallSummary{ID: tc.ID, Name: tc.Name})
	}
	out <- event.NewRequireUserConfirm(replyID, confirmID, calls)

	// Wait for the external confirmation using the saved confirmID.
	ev, err := a.waitForExternalEvent(ctx, confirmID)
	if err != nil {
		return nil, fmt.Errorf("react agent resume: wait failed: %w", err)
	}
	confirm, ok := ev.(*event.UserConfirmResultEvent)
	if !ok {
		return nil, fmt.Errorf("react agent resume: expected UserConfirmResultEvent, got %T", ev)
	}
	toolCalls = applyConfirmDecisions(toolCalls, confirm.Decisions)
	if len(toolCalls) == 0 {
		finalResponse := message.NewMsg().Role(message.RoleTool).TextContent("All tool calls were denied by user.").Build()
		_ = a.memory.Add(msg)
		_ = a.memory.Add(finalResponse)
		return finalResponse, nil
	}

	// Clear suspend state so subsequent SaveState reflects a running agent.
	a.runtimeMu.Lock()
	if a.runtimeState != nil {
		a.runtimeState.SuspendedAt = nil
		a.runtimeState.SuspendedEvent = ""
		a.runtimeState.WaitConfirmID = ""
	}
	a.runtimeMu.Unlock()

	// Bypass permission check because it was already performed before suspension.
	oldPerm := a.permissionEngine
	a.permissionEngine = nil
	toolResultMsg, err := a.executeToolsStream(ctx, history, toolCalls, out, replyID, startIter)
	a.permissionEngine = oldPerm
	if err != nil {
		return nil, err
	}
	history = append(history, toolResultMsg)

	// Perform one final reasoning round with the tool results.
	toolSpecs := a.toolSpecs(ctx)
	var chatOpts []model.ChatOption
	if len(toolSpecs) > 0 {
		chatOpts = append(chatOpts, model.WithTools(toolSpecs))
	}

	response, err := a.runModelStream(ctx, history, chatOpts, startIter+1, len(toolSpecs) > 0, out, replyID)
	if err != nil {
		return nil, err
	}

	// Before-finish hooks
	_, hr, err := a.fireHooks(ctx, hook.HookBeforeFinish, history, response, "", nil)
	if err != nil {
		return nil, err
	}
	if hr != nil && hr.Override != nil {
		response = hr.Override
	}

	// Persist to memory
	_ = a.memory.Add(msg)
	_ = a.memory.Add(response)

	// Update runtime state so Reply() can retrieve the final message.
	a.runtimeMu.Lock()
	if a.runtimeState != nil {
		finalHistory := append([]*message.Msg(nil), history...)
		finalHistory = append(finalHistory, response)
		a.runtimeState.Messages = finalHistory
		a.runtimeState.UpdatedAt = time.Now()
	}
	a.runtimeMu.Unlock()

	return response, nil
}

// runModelStream calls the model and emits fine-grained events.
// When streaming is possible (no tools requested), it emits TextBlockDeltaEvent / ThinkingBlockDeltaEvent.
func (a *ReActAgent) runModelStream(
	ctx context.Context,
	history []*message.Msg,
	chatOpts []model.ChatOption,
	iter int,
	requestTools bool,
	out chan<- event.AgentEvent,
	replyID string,
) (*message.Msg, error) {
	// PreReasoning event via existing hook system
	pre := &hook.PreReasoningEvent{
		BaseEvent: hook.BaseEvent{
			Type:      hook.EventPreReasoning,
			Ts:        time.Now(),
			Agent:     a.Base.Name,
			Iteration: iter,
		},
		Messages:  append([]*message.Msg(nil), history...),
		ModelName: a.chatModel.ModelName(),
		ChatOpts:  chatOpts,
	}
	if ev, _, err := a.fireStreamEvent(ctx, pre); err != nil {
		return nil, err
	} else if preEv, ok := ev.(*hook.PreReasoningEvent); ok {
		chatOpts = preEv.ChatOpts
	}

	modelName := a.chatModel.ModelName()
	out <- event.NewModelCallStart(replyID, modelName)

	// When tools are requested, we must use Chat (non-streaming) to guarantee
	// correct tool-call parsing. Emit a single text block after completion.
	if requestTools {
		msg, err := a.chatModel.Chat(ctx, history, chatOpts...)
		if err != nil {
			out <- event.NewError(replyID, fmt.Errorf("react agent model call: %w", err))
			out <- event.NewModelCallEnd(replyID, modelName, 0, 0)
			return nil, fmt.Errorf("react agent model call: %w", err)
		}
		out <- event.NewTextBlockStart(replyID, 0)
		out <- event.NewTextBlockDelta(replyID, 0, msg.GetTextContent())
		out <- event.NewTextBlockEnd(replyID, 0)
		u := extractUsage(msg)
		out <- event.NewModelCallEnd(replyID, modelName, u.PromptTokens, u.CompletionTokens)
		_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
			BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
			Messages:  append([]*message.Msg(nil), history...),
			Response:  msg,
		})
		return msg, nil
	}

	// Streaming path: emit deltas as they arrive.
	ch, err := a.chatModel.ChatStream(ctx, history, chatOpts...)
	if err != nil {
		out <- event.NewError(replyID, fmt.Errorf("react agent model stream: %w", err))
		out <- event.NewModelCallEnd(replyID, modelName, 0, 0)
		return nil, fmt.Errorf("react agent model stream: %w", err)
	}

	var (
		sb              strings.Builder
		thinkingSb      strings.Builder
		streamUsage     *model.ChatUsage
		inTextBlock     bool
		inThinkingBlock bool
	)
	for chunk := range ch {
		if chunk == nil {
			continue
		}
		if chunk.Done {
			if chunk.Usage != nil {
				streamUsage = chunk.Usage
			}
			break
		}
		if chunk.Delta == "" {
			continue
		}

		if chunk.IsThinking {
			if inTextBlock {
				out <- event.NewTextBlockEnd(replyID, 0)
				inTextBlock = false
			}
			if !inThinkingBlock {
				out <- event.NewThinkingBlockStart(replyID, 0)
				inThinkingBlock = true
			}
			thinkingSb.WriteString(chunk.Delta)
			out <- event.NewThinkingBlockDelta(replyID, 0, chunk.Delta)
		} else {
			if inThinkingBlock {
				out <- event.NewThinkingBlockEnd(replyID, 0)
				inThinkingBlock = false
			}
			if !inTextBlock {
				out <- event.NewTextBlockStart(replyID, 0)
				inTextBlock = true
			}
			sb.WriteString(chunk.Delta)
			out <- event.NewTextBlockDelta(replyID, 0, chunk.Delta)
		}
	}
	if inThinkingBlock {
		out <- event.NewThinkingBlockEnd(replyID, 0)
	}
	if inTextBlock {
		out <- event.NewTextBlockEnd(replyID, 0)
	}

	msg := message.NewMsg().Role(message.RoleAssistant).TextContent(sb.String()).Build()
	if thinkingSb.Len() > 0 {
		msg.Content = append(msg.Content, message.NewThinkingBlock(thinkingSb.String(), ""))
	}
	if streamUsage != nil {
		msg.Metadata["usage"] = *streamUsage
	}
	inputTokens, outputTokens := 0, 0
	if streamUsage != nil {
		inputTokens = streamUsage.PromptTokens
		outputTokens = streamUsage.CompletionTokens
	}
	out <- event.NewModelCallEnd(replyID, modelName, inputTokens, outputTokens)
	_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
		Messages:  append([]*message.Msg(nil), history...),
		Response:  msg,
	})
	return msg, nil
}

// executeToolsStream executes tool calls concurrently and emits ToolResult* events.
//
//nolint:unparam
func (a *ReActAgent) executeToolsStream(
	ctx context.Context,
	history []*message.Msg,
	toolCalls []*message.ToolUseBlock,
	out chan<- event.AgentEvent,
	replyID string,
	iter int,
) (*message.Msg, error) {
	// V2: permission check before executing tools
	if a.permissionEngine != nil {
		evals, err := a.permissionEngine.Evaluate(toolCalls)
		if err != nil {
			return nil, err
		}
		var asking []event.ToolCallSummary
		for _, ev := range evals {
			if ev.Decision == permission.DecisionDeny {
				return nil, fmt.Errorf("permission denied for %s: %s", ev.ToolName, ev.Message)
			}
			if ev.Decision == permission.DecisionAsk {
				asking = append(asking, event.ToolCallSummary{
					ID:    ev.ToolCallID,
					Name:  ev.ToolName,
					Input: findToolInput(toolCalls, ev.ToolCallID),
				})
			}
		}
		if len(asking) > 0 {
			confirmID := uuid.New().String()
			out <- event.NewRequireUserConfirm(replyID, confirmID, asking)

			// Save suspend state for potential reconnect resume
			tnow := time.Now()
			a.runtimeMu.Lock()
			if a.runtimeState != nil {
				a.runtimeState.SuspendedAt = &tnow
				a.runtimeState.SuspendedEvent = event.TypeRequireUserConfirm
				a.runtimeState.WaitConfirmID = confirmID
			}
			a.runtimeMu.Unlock()

			// Suspend: wait for external UserConfirmResultEvent
			ev, err := a.waitForExternalEvent(ctx, confirmID)
			if err != nil {
				return nil, fmt.Errorf("permission confirmation wait: %w", err)
			}
			confirm, ok := ev.(*event.UserConfirmResultEvent)
			if !ok {
				return nil, fmt.Errorf("expected UserConfirmResultEvent, got %T", ev)
			}

			// Clear suspend state after resume
			a.runtimeMu.Lock()
			if a.runtimeState != nil {
				a.runtimeState.SuspendedAt = nil
				a.runtimeState.SuspendedEvent = ""
				a.runtimeState.WaitConfirmID = ""
			}
			a.runtimeMu.Unlock()

			// Apply decisions: filter out denied tool calls, apply modifications
			toolCalls = applyConfirmDecisions(toolCalls, confirm.Decisions)
			if len(toolCalls) == 0 {
				return message.NewMsg().Role(message.RoleTool).TextContent("All tool calls were denied by user.").Build(), nil
			}
		}
	}

	type result struct {
		blocks     []message.ContentBlock
		resultMsg  *message.Msg
		toolName   string
		toolInput  map[string]any
		toolCallID string
		tcr        memory.ToolCallResult
		hasTcr     bool
		elapsed    float64
		err        error
	}

	results := make([]result, len(toolCalls))
	externalDone := make(map[int]bool)

	// External tools: suspend until client executes and injects results.
	var externalCalls []event.ToolCallSummary
	externalIdx := map[string]int{}
	for idx, tc := range toolCalls {
		if a.isExternalTool(ctx, tc.Name) {
			externalCalls = append(externalCalls, event.ToolCallSummary{
				ID: tc.ID, Name: tc.Name, Input: tc.Input,
			})
			externalIdx[tc.ID] = idx
		}
	}
	if len(externalCalls) > 0 {
		confirmID := uuid.New().String()
		out <- event.NewRequireExternalExecution(replyID, confirmID, externalCalls)
		tnow := time.Now()
		a.runtimeMu.Lock()
		if a.runtimeState != nil {
			a.runtimeState.SuspendedAt = &tnow
			a.runtimeState.SuspendedEvent = event.TypeRequireExternalExecution
			a.runtimeState.WaitConfirmID = confirmID
		}
		a.runtimeMu.Unlock()

		ev, err := a.waitForExternalEvent(ctx, confirmID)
		if err != nil {
			return nil, fmt.Errorf("external execution wait: %w", err)
		}
		a.runtimeMu.Lock()
		if a.runtimeState != nil {
			a.runtimeState.SuspendedAt = nil
			a.runtimeState.SuspendedEvent = ""
			a.runtimeState.WaitConfirmID = ""
		}
		a.runtimeMu.Unlock()

		ext, ok := ev.(*event.ExternalExecutionResultEvent)
		if !ok {
			return nil, fmt.Errorf("expected ExternalExecutionResultEvent, got %T", ev)
		}
		byID := map[string]event.ExternalExecutionResult{}
		for _, r := range ext.Results {
			byID[r.ToolCallID] = r
		}
		for _, tc := range externalCalls {
			idx := externalIdx[tc.ID]
			out <- event.NewToolCallStart(replyID, idx, tc.ID, tc.Name)
			out <- event.NewToolCallEnd(replyID, idx, tc.ID)
			out <- event.NewToolResultStart(replyID, idx, tc.ID, tc.Name)

			r, found := byID[tc.ID]
			var blocks []message.ContentBlock
			toolErr := false
			if !found || !r.Success {
				toolErr = true
				msg := r.Error
				if msg == "" {
					msg = "external execution failed"
				}
				blocks = []message.ContentBlock{message.NewTextBlock(msg)}
			} else {
				blocks = []message.ContentBlock{message.NewTextBlock(r.Output)}
			}
			blocks = a.compressToolResultBlocks(ctx, tc.ID, blocks, toolErr)
			var resultText string
			for _, b := range blocks {
				if tb, ok := b.(*message.TextBlock); ok {
					resultText += tb.Text
				}
			}
			if resultText != "" {
				out <- event.NewToolResultTextDelta(replyID, idx, tc.ID, resultText)
			}
			out <- event.NewToolResultEnd(replyID, idx, tc.ID)
			results[idx] = result{
				blocks:     blocks,
				resultMsg:  message.NewMsg().Role(message.RoleTool).Content(message.NewToolResultBlock(tc.ID, blocks, toolErr)).Build(),
				toolName:   tc.Name,
				toolInput:  tc.Input,
				toolCallID: tc.ID,
			}
			externalDone[idx] = true
		}
	}

	var g sync.WaitGroup

	for idx, tc := range toolCalls {
		if externalDone[idx] {
			continue
		}
		tc := tc
		idx := idx
		g.Add(1)
		go func() {
			defer g.Done()

			out <- event.NewToolCallStart(replyID, idx, tc.ID, tc.Name)
			out <- event.NewToolCallEnd(replyID, idx, tc.ID)

			// Fire before-tool hook
			_, hr, err := a.fireHooks(ctx, hook.HookBeforeTool, history, nil, tc.Name, tc.Input)
			if err != nil {
				results[idx] = result{err: err, toolCallID: tc.ID}
				return
			}
			if hr != nil && hr.Interrupt {
				results[idx] = result{err: &hookInterruptError{override: hr.Override}, toolCallID: tc.ID}
				return
			}

			out <- event.NewToolResultStart(replyID, idx, tc.ID, tc.Name)

			start := time.Now()
			resp, toolErr := a.executeTool(ctx, tc.Name, tc.Input)
			elapsed := time.Since(start).Seconds()

			var blocks []message.ContentBlock
			if toolErr != nil {
				blocks = []message.ContentBlock{message.NewTextBlock(fmt.Sprintf("error: %s", toolErr.Error()))}
			} else if resp != nil && len(resp.Content) > 0 {
				blocks = resp.Content
			} else {
				blocks = []message.ContentBlock{message.NewTextBlock("")}
			}
			blocks = a.compressToolResultBlocks(ctx, tc.ID, blocks, toolErr != nil)

			// Emit tool result text delta (aggregated for now; future: per-chunk)
			var resultText string
			for _, b := range blocks {
				if tb, ok := b.(*message.TextBlock); ok {
					resultText += tb.Text
				}
			}
			if resultText != "" {
				out <- event.NewToolResultTextDelta(replyID, idx, tc.ID, resultText)
			}

			// Emit data deltas for binary blocks (image, audio, video, data)
			for _, b := range blocks {
				switch d := b.(type) {
				case *message.ImageBlock:
					data := d.Base64
					if data == "" {
						data = d.URL
					}
					out <- event.NewToolResultDataDelta(replyID, idx, tc.ID, data, d.MimeType)
				case *message.AudioBlock:
					data := d.Base64
					if data == "" {
						data = d.URL
					}
					out <- event.NewToolResultDataDelta(replyID, idx, tc.ID, data, d.MimeType)
				case *message.VideoBlock:
					out <- event.NewToolResultDataDelta(replyID, idx, tc.ID, d.URL, "video/*")
				case *message.DataBlock:
					if d.Source != nil {
						out <- event.NewToolResultDataDelta(replyID, idx, tc.ID, d.Source.Data, d.Source.MediaType)
					}
				}
			}
			out <- event.NewToolResultEnd(replyID, idx, tc.ID)

			resultMsg := message.NewMsg().Role(message.RoleTool).Content(
				message.NewToolResultBlock(tc.ID, blocks, toolErr != nil),
			).Build()

			// After-tool hook
			_, afterHr, afterErr := a.fireHooks(ctx, hook.HookAfterTool, history, nil, tc.Name, tc.Input)
			if afterErr != nil {
				results[idx] = result{err: afterErr, toolCallID: tc.ID}
				return
			}
			if afterHr != nil && (afterHr.StopAgent || afterHr.Interrupt) {
				results[idx] = result{
					err:        &hookInterruptError{override: afterHr.Override},
					resultMsg:  resultMsg,
					toolCallID: tc.ID,
				}
				return
			}

			var hasTcr bool
			var tcr memory.ToolCallResult
			if collector, ok := a.memory.(interface {
				AddToolCallResult(ctx context.Context, result memory.ToolCallResult) error
			}); ok {
				outputText := ""
				for _, b := range blocks {
					if tb, ok := b.(*message.TextBlock); ok {
						outputText += tb.Text
					}
				}
				tcr = memory.ToolCallResult{
					ToolName: tc.Name,
					Input:    tc.Input,
					Output:   outputText,
					Success:  toolErr == nil,
					TimeCost: elapsed,
				}
				hasTcr = true
				_ = collector.AddToolCallResult(ctx, tcr)
			}

			results[idx] = result{
				blocks:     blocks,
				resultMsg:  resultMsg,
				toolName:   tc.Name,
				toolInput:  tc.Input,
				toolCallID: tc.ID,
				tcr:        tcr,
				hasTcr:     hasTcr,
				elapsed:    elapsed,
				err:        toolErr,
			}
		}()
	}

	g.Wait()

	// Check for any interrupt errors
	for _, r := range results {
		if r.err != nil {
			if hi, ok := r.err.(*hookInterruptError); ok {
				return hi.override, nil
			}
			return nil, r.err
		}
	}

	toolResultMsg := message.NewMsg().Role(message.RoleTool)
	for _, r := range results {
		toolResultMsg.Content(r.resultMsg.Content...)
	}

	// Batch summarize
	if collector, ok := a.memory.(interface {
		SummarizeToolUsage(ctx context.Context, toolName string) error
	}); ok {
		called := make(map[string]bool)
		for _, r := range results {
			if r.hasTcr {
				called[r.toolName] = true
			}
		}
		for name := range called {
			_ = collector.SummarizeToolUsage(ctx, name)
		}
	}

	return toolResultMsg.Build(), nil
}

// findToolInput returns the input map for a tool call by ID.
func findToolInput(calls []*message.ToolUseBlock, id string) map[string]any {
	for _, c := range calls {
		if c.ID == id {
			return c.Input
		}
	}
	return nil
}

// applyConfirmDecisions filters and modifies tool calls based on user confirmation decisions.
func applyConfirmDecisions(calls []*message.ToolUseBlock, decisions []event.ConfirmDecision) []*message.ToolUseBlock {
	decisionMap := make(map[string]event.ConfirmDecision, len(decisions))
	for _, d := range decisions {
		decisionMap[d.ToolCallID] = d
	}

	var result []*message.ToolUseBlock
	for _, c := range calls {
		d, ok := decisionMap[c.ID]
		if !ok {
			// No decision for this tool call — default to deny for safety
			continue
		}
		switch d.Decision {
		case "allow":
			result = append(result, c)
		case "modify":
			if len(d.ModifiedArgs) > 0 {
				c.Input = d.ModifiedArgs
			}
			result = append(result, c)
		case "deny":
			// Skip
		default:
			// Unknown decision — skip for safety
		}
	}
	return result
}
