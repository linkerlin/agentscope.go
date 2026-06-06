package a2a

import (
	"context"

	"github.com/linkerlin/agentscope.go/agent"
	"github.com/linkerlin/agentscope.go/event"
)

// V2AgentAdapter wraps an agent.V2Agent to implement StreamingAgentRunner
// with fine-grained event streaming (aligned with PyV2 Studio UI protocol).
type V2AgentAdapter struct {
	agent agent.V2Agent
}

// NewV2AgentAdapter creates a new V2 adapter.
func NewV2AgentAdapter(a agent.V2Agent) *V2AgentAdapter {
	return &V2AgentAdapter{agent: a}
}

// Run implements AgentRunner.
func (a *V2AgentAdapter) Run(ctx context.Context, msg *Message) (*Message, error) {
	resp, err := a.agent.Call(ctx, toMsg(msg))
	if err != nil {
		return nil, err
	}
	return fromMsg(resp), nil
}

// EventStreamMessage is an A2A message enriched with V2 event metadata.
type EventStreamMessage struct {
	Message
	EventType string `json:"event_type,omitempty"` // e.g. "text_block_delta"
	BlockIndex int   `json:"block_index,omitempty"`
	Delta     string `json:"delta,omitempty"`
}

// RunStream implements StreamingAgentRunner using ReplyStream.
// It converts V2 AgentEvents into A2A messages for cross-agent / UI consumption.
func (a *V2AgentAdapter) RunStream(ctx context.Context, msg *Message) (<-chan *Message, error) {
	evCh, err := a.agent.ReplyStream(ctx, toMsg(msg))
	if err != nil {
		return nil, err
	}

	out := make(chan *Message, 64)
	go func() {
		defer close(out)
		var textBuf string
		for ev := range evCh {
			if ev == nil {
				continue
			}
			switch e := ev.(type) {
			case *event.TextBlockDeltaEvent:
				textBuf += e.Delta
				out <- &Message{
					Role:    "agent",
					Content: textBuf,
					Meta: map[string]any{
						"event_type": "text_block_delta",
						"delta":      e.Delta,
						"partial":    true,
					},
				}
			case *event.ThinkingBlockDeltaEvent:
				out <- &Message{
					Role:    "agent",
					Content: "",
					Meta: map[string]any{
						"event_type": "thinking_block_delta",
						"delta":      e.Delta,
						"partial":    true,
					},
				}
			case *event.ToolCallStartEvent:
				out <- &Message{
					Role:    "agent",
					Content: "",
					Meta: map[string]any{
						"event_type": "tool_call_start",
						"tool_name":  e.ToolName,
						"tool_call_id": e.ToolCallID,
					},
				}
			case *event.ToolResultTextDeltaEvent:
				out <- &Message{
					Role:    "agent",
					Content: e.Delta,
					Meta: map[string]any{
						"event_type":   "tool_result_delta",
						"tool_call_id": e.ToolCallID,
						"partial":      true,
					},
				}
			case *event.RequireUserConfirmEvent:
				out <- &Message{
					Role:    "agent",
					Content: "Waiting for user confirmation...",
					Meta: map[string]any{
						"event_type": "require_user_confirm",
						"confirm_id": e.ConfirmID,
						"tool_calls": e.ToolCalls,
					},
				}
			case *event.ReplyEndEvent:
				out <- &Message{
					Role:    "agent",
					Content: textBuf,
					Meta: map[string]any{
						"event_type": "reply_end",
						"final":      true,
					},
				}
			case *event.ErrorEvent:
				out <- &Message{
					Role:    "agent",
					Content: e.Err,
					Meta: map[string]any{
						"event_type": "error",
					},
				}
			}
		}
	}()
	return out, nil
}

var _ AgentRunner = (*V2AgentAdapter)(nil)
var _ StreamingAgentRunner = (*V2AgentAdapter)(nil)
