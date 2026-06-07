package agent

import (
	"errors"
	"strings"

	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
)

// CollectMessage drains an event stream and reconstructs the final assistant
// message from TextBlockDelta and ThinkingBlockDelta events.
// It returns when the channel is closed. If an ErrorEvent is encountered,
// it is returned as an error.
//
// Note: This is a best-effort reconstruction. Tool calls and metadata
// (usage, etc.) are not recovered from the event stream because they are
// not emitted as V2 events. For full fidelity, use Reply() instead.
func CollectMessage(ch <-chan event.AgentEvent) (*message.Msg, error) {
	var (
		sb         strings.Builder
		thinkingSb strings.Builder
		lastErr    error
	)
	for ev := range ch {
		switch e := ev.(type) {
		case *event.TextBlockDeltaEvent:
			sb.WriteString(e.Delta)
		case *event.ThinkingBlockDeltaEvent:
			thinkingSb.WriteString(e.Delta)
		case *event.ErrorEvent:
			if e.Err != "" {
				lastErr = errors.New(e.Err)
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	var blocks []message.ContentBlock
	if sb.Len() > 0 {
		blocks = append(blocks, message.NewTextBlock(sb.String()))
	}
	if thinkingSb.Len() > 0 {
		blocks = append(blocks, message.NewThinkingBlock(thinkingSb.String(), ""))
	}
	msg := message.NewMsg().Role(message.RoleAssistant).Content(blocks...).Build()
	return msg, nil
}
