package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/linkerlin/agentscope.go/event"
)

// AGUIConvertOptions carries session context for AG-UI event mapping.
type AGUIConvertOptions struct {
	ThreadID string // maps to AG-UI threadId (typically session_id)
}

// AGUIConverter converts AgentScope AgentEvent values to AG-UI protocol events.
type AGUIConverter interface {
	Convert(ev event.AgentEvent, opts AGUIConvertOptions) (map[string]any, error)
}

// DefaultAGUIConverter implements the mapping aligned with PyV2 _agui.py.
type DefaultAGUIConverter struct {
	mu                 sync.Mutex
	lastModelName      string
	toolResultBuffers  map[string][]string
}

// NewDefaultAGUIConverter creates an AG-UI converter.
func NewDefaultAGUIConverter() *DefaultAGUIConverter {
	return &DefaultAGUIConverter{
		toolResultBuffers: make(map[string][]string),
	}
}

func blockMessageID(replyID string, blockIndex int) string {
	return fmt.Sprintf("%s:%d", replyID, blockIndex)
}

// Convert maps a single AgentEvent to an AG-UI protocol event dictionary.
func (c *DefaultAGUIConverter) Convert(ev event.AgentEvent, opts AGUIConvertOptions) (map[string]any, error) {
	if ev == nil {
		return nil, fmt.Errorf("agui: nil event")
	}

	switch e := ev.(type) {
	case *event.ReplyStartEvent:
		return map[string]any{
			"type":     "RUN_STARTED",
			"threadId": opts.ThreadID,
			"runId":    e.ReplyID(),
		}, nil

	case *event.ReplyEndEvent:
		return map[string]any{
			"type":     "RUN_FINISHED",
			"threadId": opts.ThreadID,
			"runId":    e.ReplyID(),
		}, nil

	case *event.ExceedMaxItersEvent:
		return map[string]any{
			"type":    "RUN_ERROR",
			"message": fmt.Sprintf("Agent exceeded max iterations (%d)", e.MaxIters),
			"code":    "exceed_max_iters",
		}, nil

	case *event.ModelCallStartEvent:
		c.mu.Lock()
		c.lastModelName = e.ModelName
		c.mu.Unlock()
		return map[string]any{
			"type":     "STEP_STARTED",
			"stepName": e.ModelName,
		}, nil

	case *event.ModelCallEndEvent:
		c.mu.Lock()
		step := c.lastModelName
		if step == "" {
			step = "model_call"
		}
		c.mu.Unlock()
		return map[string]any{
			"type":     "STEP_FINISHED",
			"stepName": step,
		}, nil

	case *event.TextBlockStartEvent:
		return map[string]any{
			"type":      "TEXT_MESSAGE_START",
			"messageId": blockMessageID(e.ReplyID(), e.BlockIndex),
		}, nil

	case *event.TextBlockDeltaEvent:
		return map[string]any{
			"type":      "TEXT_MESSAGE_CONTENT",
			"messageId": blockMessageID(e.ReplyID(), e.BlockIndex),
			"delta":     e.Delta,
		}, nil

	case *event.TextBlockEndEvent:
		return map[string]any{
			"type":      "TEXT_MESSAGE_END",
			"messageId": blockMessageID(e.ReplyID(), e.BlockIndex),
		}, nil

	case *event.ThinkingBlockStartEvent:
		return map[string]any{
			"type":      "REASONING_MESSAGE_START",
			"messageId": blockMessageID(e.ReplyID(), e.BlockIndex),
			"role":      "reasoning",
		}, nil

	case *event.ThinkingBlockDeltaEvent:
		return map[string]any{
			"type":      "REASONING_MESSAGE_CONTENT",
			"messageId": blockMessageID(e.ReplyID(), e.BlockIndex),
			"delta":     e.Delta,
		}, nil

	case *event.ThinkingBlockEndEvent:
		return map[string]any{
			"type":      "REASONING_MESSAGE_END",
			"messageId": blockMessageID(e.ReplyID(), e.BlockIndex),
		}, nil

	case *event.ToolCallStartEvent:
		return map[string]any{
			"type":            "TOOL_CALL_START",
			"toolCallId":      e.ToolCallID,
			"toolCallName":    e.ToolName,
			"parentMessageId": e.ReplyID(),
		}, nil

	case *event.ToolCallDeltaEvent:
		return map[string]any{
			"type":       "TOOL_CALL_ARGS",
			"toolCallId": e.ToolCallID,
			"delta":      e.Delta,
		}, nil

	case *event.ToolCallEndEvent:
		return map[string]any{
			"type":       "TOOL_CALL_END",
			"toolCallId": e.ToolCallID,
		}, nil

	case *event.ToolResultStartEvent:
		return c.customEvent("tool_result_start", e)

	case *event.ToolResultTextDeltaEvent:
		c.mu.Lock()
		c.toolResultBuffers[e.ToolCallID] = append(c.toolResultBuffers[e.ToolCallID], e.Delta)
		c.mu.Unlock()
		return c.customEvent("tool_result_text_delta", e)

	case *event.ToolResultDataDeltaEvent:
		return c.customEvent("tool_result_data_delta", e)

	case *event.ToolResultEndEvent:
		c.mu.Lock()
		content := ""
		if parts, ok := c.toolResultBuffers[e.ToolCallID]; ok {
			for _, p := range parts {
				content += p
			}
			delete(c.toolResultBuffers, e.ToolCallID)
		}
		c.mu.Unlock()
		return map[string]any{
			"type":       "TOOL_CALL_RESULT",
			"toolCallId": e.ToolCallID,
			"messageId":  e.ReplyID(),
			"content":    content,
		}, nil

	case *event.DataBlockStartEvent:
		return c.customEvent("data_block_start", e)
	case *event.DataBlockDeltaEvent:
		return c.customEvent("data_block_delta", e)
	case *event.DataBlockEndEvent:
		return c.customEvent("data_block_end", e)
	case *event.RequireUserConfirmEvent:
		return c.customEvent("require_user_confirm", e)
	case *event.RequireExternalExecutionEvent:
		return c.customEvent("require_external_execution", e)
	case *event.UserConfirmResultEvent:
		return c.customEvent("user_confirm_result", e)
	case *event.ExternalExecutionResultEvent:
		return c.customEvent("external_execution_result", e)
	case *event.ErrorEvent:
		return map[string]any{
			"type":    "RUN_ERROR",
			"message": e.Err,
			"code":    "agent_error",
		}, nil
	default:
		return c.customEvent("unknown", ev)
	}
}

func (c *DefaultAGUIConverter) customEvent(name string, ev event.AgentEvent) (map[string]any, error) {
	payload, err := event.MarshalEvent(ev)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	return map[string]any{
		"type":  "CUSTOM",
		"name":  name,
		"value": value,
	}, nil
}

// EncodeEvent marshals a single event using this converter instance.
// Reuse one converter per SSE/WS stream so tool-result buffers stay consistent.
func (c *DefaultAGUIConverter) EncodeEvent(ev event.AgentEvent, opts AGUIConvertOptions) ([]byte, error) {
	aguiEv, err := c.Convert(ev, opts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(aguiEv)
}

// EncodeStreamEvent serializes an AgentEvent for SSE/WS transport.
// When useAGUI is true, pass a shared *DefaultAGUIConverter for the stream.
func EncodeStreamEvent(ev event.AgentEvent, opts AGUIConvertOptions, useAGUI bool, conv *DefaultAGUIConverter) ([]byte, error) {
	if useAGUI {
		if conv == nil {
			conv = NewDefaultAGUIConverter()
		}
		return conv.EncodeEvent(ev, opts)
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v2Event{
		EventType: ev.EventType(),
		Timestamp: ev.Timestamp().Format("2006-01-02T15:04:05.000Z"),
		ReplyID:   ev.ReplyID(),
		Payload:   payload,
	})
}
func useAGUIProtocol(r *http.Request) bool {
	if r == nil {
		return false
	}
	switch r.URL.Query().Get("protocol") {
	case "agui", "ag-ui", "AG-UI":
		return true
	}
	return r.Header.Get("X-Protocol") == "agui"
}
