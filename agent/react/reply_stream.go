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

	replyID := uuid.New().String()
	out <- event.NewReplyStart(replyID, a.Name())

	// Initialize runtime state for this reply
	a.runtimeMu.Lock()
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
	a.runtimeMu.Unlock()

	defer func() {
		out <- event.NewReplyEnd(replyID, a.Name())
	}()

	// For backward compatibility with Base.Call lifecycle, we keep the wrapper
	// but the internal loop emits events directly.
	_, err := a.Base.Call(ctx, msg, func(innerCtx context.Context, input *message.Msg) (*message.Msg, error) {
		return a.replyStreamInternal(innerCtx, input, out, replyID)
	})
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

	toolSpecs := a.toolSpecs()
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
		err := errors.New("react agent: max iterations reached without final answer")
		out <- event.NewError(replyID, err)
		return nil, err
	}

	// Persist to memory
	_ = a.memory.Add(msg)
	_ = a.memory.Add(finalResponse)

	return finalResponse, nil
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

	// When tools are requested, we must use Chat (non-streaming) to guarantee
	// correct tool-call parsing. Emit a single text block after completion.
	if requestTools {
		msg, err := a.chatModel.Chat(ctx, history, chatOpts...)
		if err != nil {
			out <- event.NewError(replyID, fmt.Errorf("react agent model call: %w", err))
			return nil, fmt.Errorf("react agent model call: %w", err)
		}
		out <- event.NewTextBlockStart(replyID, 0)
		out <- event.NewTextBlockDelta(replyID, 0, msg.GetTextContent())
		out <- event.NewTextBlockEnd(replyID, 0)
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
	_, _, _ = a.fireStreamEvent(ctx, &hook.PostReasoningEvent{
		BaseEvent: hook.BaseEvent{Type: hook.EventPostReasoning, Ts: time.Now(), Agent: a.Base.Name, Iteration: iter},
		Messages:  append([]*message.Msg(nil), history...),
		Response:  msg,
	})
	return msg, nil
}

// executeToolsStream executes tool calls concurrently and emits ToolResult* events.
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
				return nil, fmt.Errorf("permission denied for tool %s: %s", ev.ToolName, ev.Reason)
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

			// Suspend: wait for external UserConfirmResultEvent
			ev, err := a.waitForExternalEvent(ctx, confirmID)
			if err != nil {
				return nil, fmt.Errorf("permission confirmation wait: %w", err)
			}
			confirm, ok := ev.(*event.UserConfirmResultEvent)
			if !ok {
				return nil, fmt.Errorf("expected UserConfirmResultEvent, got %T", ev)
			}
			// Apply decisions: filter out denied tool calls, apply modifications
			toolCalls = applyConfirmDecisions(toolCalls, confirm.Decisions)
			if len(toolCalls) == 0 {
				return message.NewMsg().Role(message.RoleTool).TextContent("All tool calls were denied by user.").Build(), nil
			}
		}
	}

	type result struct {
		blocks      []message.ContentBlock
		resultMsg   *message.Msg
		toolName    string
		toolInput   map[string]any
		toolCallID  string
		tcr         memory.ToolCallResult
		hasTcr      bool
		elapsed     float64
		err         error
	}

	results := make([]result, len(toolCalls))
	var g sync.WaitGroup

	for idx, tc := range toolCalls {
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
