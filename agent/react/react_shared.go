package react

import (
	"context"
	"errors"
	"fmt"

	"github.com/linkerlin/agentscope.go/hook"
	"github.com/linkerlin/agentscope.go/message"
)

// errEmptyModelResponse marks a model turn that produced no content at all:
// no text, no thinking and no tool calls. It must surface as an error instead
// of a silent empty reply (PyV2 #1861).
var errEmptyModelResponse = errors.New("react agent: model returned an empty response")

// isDegenerateResponse reports whether a model response carries no usable
// content (nil or zero blocks).
func isDegenerateResponse(msg *message.Msg) bool {
	return msg == nil || len(msg.Content) == 0
}

// loopAction controls ReAct loop flow after hook evaluation.
type loopAction int

const (
	loopNormal   loopAction = iota // continue normally
	loopBreak                      // break with override (or nil)
	loopContinue                   // skip rest of iteration (goto reasoning)
	loopReturn                     // return immediately
)

// preCallPhase fires the PreCall hook and returns the effective input message.
// If a hook produced an override or interrupt, returns (nil, overrideMsg, nil).
func (a *ReActAgent) preCallPhase(ctx context.Context, msg *message.Msg) (*message.Msg, *message.Msg, error) {
	preCallMsgs, hr, err := a.fireHooks(ctx, hook.HookPreCall, []*message.Msg{msg}, nil, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if hr != nil && (hr.Interrupt || hr.Override != nil) {
		return nil, hr.Override, nil
	}
	inputMsg := msg
	if len(preCallMsgs) > 0 {
		inputMsg = preCallMsgs[0]
	}
	return inputMsg, nil, nil
}

// loopGuard checks interrupt flag and closed state at the start of each iteration.
// pendingToolCalls are passed to handleInterrupt if interrupted.
func (a *ReActAgent) loopGuard(ctx context.Context, originalMsg *message.Msg, pendingToolCalls []*message.ToolUseBlock) (*message.Msg, error) {
	if err := a.CheckInterrupted(); err != nil {
		return a.handleInterrupt(ctx, originalMsg, nil, pendingToolCalls)
	}
	a.Mu.RLock()
	closed := a.Closed
	a.Mu.RUnlock()
	if closed {
		return nil, ErrAgentClosed
	}
	return nil, nil
}

// beforeModelPhase fires BeforeModel hooks and returns the action to take.
// Possible actions: loopNormal (proceed), loopBreak (stop with override), loopReturn (return override).
func (a *ReActAgent) beforeModelPhase(ctx context.Context, history []*message.Msg) ([]*message.Msg, loopAction, *message.Msg, error) {
	updated, hr, err := a.fireHooks(ctx, hook.HookBeforeModel, history, nil, "", nil)
	if err != nil {
		return history, loopReturn, nil, err
	}
	if hr != nil && hr.Interrupt {
		return updated, loopReturn, hr.Override, nil
	}
	if hr != nil && hr.Override != nil {
		return updated, loopBreak, hr.Override, nil
	}
	return updated, loopNormal, nil, nil
}

// afterModelPhase fires AfterModel hooks and returns the action and (possibly overridden) response.
// Possible actions: loopNormal, loopBreak (with override as finalResponse), loopContinue (goto reasoning), loopReturn.
func (a *ReActAgent) afterModelPhase(ctx context.Context, history []*message.Msg, response *message.Msg) ([]*message.Msg, *message.Msg, loopAction, *message.Msg, error) {
	_, hr, err := a.fireHooks(ctx, hook.HookAfterModel, history, response, "", nil)
	if err != nil {
		return history, response, loopReturn, nil, err
	}
	if hr != nil && hr.StopAgent {
		final := hr.Override
		if final == nil {
			final = response
		}
		return history, response, loopReturn, final, nil
	}
	if hr != nil && hr.GotoReasoning {
		history = append(history, response)
		history = append(history, hr.GotoReasoningMsgs...)
		return history, response, loopContinue, nil, nil
	}
	if hr != nil && hr.Interrupt {
		return history, hr.Override, loopReturn, hr.Override, nil
	}
	if hr != nil && hr.Override != nil {
		response = hr.Override
	}
	return history, response, loopNormal, nil, nil
}

// checkFinalAnswer checks if the model response contains no tool calls.
// If so, fires BeforeFinish hook and returns the (possibly overridden) response + isFinal=true.
func (a *ReActAgent) checkFinalAnswer(ctx context.Context, history []*message.Msg, response *message.Msg) (*message.Msg, bool, error) {
	if response == nil {
		// Guard the nil receiver before GetToolUseCalls (PyV2 #1861): a model
		// backend returning (nil, nil) must fail loudly, not panic the loop.
		return nil, false, fmt.Errorf("react agent: model returned a nil response")
	}
	if len(response.GetToolUseCalls()) > 0 {
		return response, false, nil
	}
	// A response carrying only thinking blocks has no user-visible answer yet.
	// Treat it as an intermediate step so reasoning models do not finish the
	// turn with an empty message (PyV2 #2120).
	if hasOnlyThinkingBlocks(response) {
		return response, false, nil
	}
	_, hr, err := a.fireHooks(ctx, hook.HookBeforeFinish, history, response, "", nil)
	if err != nil {
		return nil, false, err
	}
	if hr != nil && hr.Override != nil {
		response = hr.Override
	}
	return response, true, nil
}

// hasOnlyThinkingBlocks reports whether msg carries at least one block and all
// of them are thinking blocks.
func hasOnlyThinkingBlocks(msg *message.Msg) bool {
	if msg == nil || len(msg.Content) == 0 {
		return false
	}
	for _, block := range msg.Content {
		if _, ok := block.(*message.ThinkingBlock); !ok {
			return false
		}
	}
	return true
}
