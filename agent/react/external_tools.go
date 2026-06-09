package react

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

type externalToolResult struct {
	blocks []message.ContentBlock
	isErr  bool
}

// handleExternalToolCalls suspends for external tool execution and returns pre-filled results by index.
func (a *ReActAgent) handleExternalToolCalls(
	ctx context.Context,
	replyID string,
	toolCalls []*message.ToolUseBlock,
) (map[int]externalToolResult, error) {
	out := make(map[int]externalToolResult)
	var externalCalls []event.ToolCallSummary
	idxByID := map[string]int{}
	for idx, tc := range toolCalls {
		if tc == nil || !a.isExternalTool(ctx, tc.Name) {
			continue
		}
		externalCalls = append(externalCalls, event.ToolCallSummary{
			ID: tc.ID, Name: tc.Name, Input: tc.Input,
		})
		idxByID[tc.ID] = idx
	}
	if len(externalCalls) == 0 {
		return out, nil
	}

	confirmID := uuid.New().String()
	ev := event.NewRequireExternalExecution(replyID, confirmID, externalCalls)
	if a.eventBus != nil {
		a.eventBus.Publish(ev)
	}

	tnow := time.Now()
	a.runtimeMu.Lock()
	if a.runtimeState != nil {
		a.runtimeState.SuspendedAt = &tnow
		a.runtimeState.SuspendedEvent = event.TypeRequireExternalExecution
		a.runtimeState.WaitConfirmID = confirmID
	}
	a.runtimeMu.Unlock()

	injected, err := a.waitForExternalEvent(ctx, confirmID)
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

	ext, ok := injected.(*event.ExternalExecutionResultEvent)
	if !ok {
		return nil, fmt.Errorf("expected ExternalExecutionResultEvent, got %T", injected)
	}
	byID := map[string]event.ExternalExecutionResult{}
	for _, r := range ext.Results {
		byID[r.ToolCallID] = r
	}

	for _, tc := range externalCalls {
		idx := idxByID[tc.ID]
		r, found := byID[tc.ID]
		if !found || !r.Success {
			msg := r.Error
			if msg == "" {
				msg = "external execution failed"
			}
			out[idx] = externalToolResult{
				blocks: []message.ContentBlock{message.NewTextBlock(msg)},
				isErr:  true,
			}
			continue
		}
		out[idx] = externalToolResult{
			blocks: []message.ContentBlock{message.NewTextBlock(r.Output)},
			isErr:  false,
		}
	}
	return out, nil
}
