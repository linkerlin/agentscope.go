package message

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/linkerlin/agentscope.go/event"
)

// AppendEvent applies a V2 event to the message, incrementally building its
// content. It mirrors Python AgentScope v2's Msg.append_event() semantics.
//
// The method is safe to call with nil events (no-op). It returns the receiver
// for chaining.
func (m *Msg) AppendEvent(ev event.AgentEvent) *Msg {
	if ev == nil {
		return m
	}

	switch e := ev.(type) {
	// Lifecycle events
	case *event.ReplyStartEvent:
		// No-op: Msg is already created before the event stream begins.
	case *event.ReplyEndEvent:
		t := time.Now()
		m.FinishedAt = &t
	case *event.ModelCallEndEvent:
		if m.Usage == nil {
			m.Usage = &TokenUsage{}
		}
		m.Usage.PromptTokens += e.InputTokens
		m.Usage.CompletionTokens += e.OutputTokens
		m.Usage.TotalTokens += e.InputTokens + e.OutputTokens

	// Text blocks
	case *event.TextBlockStartEvent:
		m.Content = append(m.Content, NewTextBlock(""))
	case *event.TextBlockDeltaEvent:
		tb := m.findTextBlock(e.BlockIndex)
		if tb == nil {
			m.Content = append(m.Content, NewTextBlock(""))
			tb = m.findTextBlock(e.BlockIndex)
		}
		if tb != nil {
			tb.Text += e.Delta
		}
	case *event.TextBlockEndEvent:
		// No-op

	// Thinking blocks
	case *event.ThinkingBlockStartEvent:
		m.Content = append(m.Content, NewThinkingBlock("", ""))
	case *event.ThinkingBlockDeltaEvent:
		tb := m.findThinkingBlock(e.BlockIndex)
		if tb == nil {
			m.Content = append(m.Content, NewThinkingBlock("", ""))
			tb = m.findThinkingBlock(e.BlockIndex)
		}
		if tb != nil {
			tb.Thinking += e.Delta
		}
	case *event.ThinkingBlockEndEvent:
		// No-op

	// Hint blocks
	case *event.HintBlockStartEvent:
		m.Content = append(m.Content, NewHintBlock("", ""))
	case *event.HintBlockDeltaEvent:
		hb := m.findHintBlock(e.BlockIndex)
		if hb == nil {
			m.Content = append(m.Content, NewHintBlock("", ""))
			hb = m.findHintBlock(e.BlockIndex)
		}
		if hb != nil {
			hb.Text += e.Delta
		}
	case *event.HintBlockEndEvent:
		// No-op

	// Data blocks
	case *event.DataBlockStartEvent:
		src := &Source{MediaType: e.MediaType}
		m.Content = append(m.Content, NewDataBlock(TypeData, src))
	case *event.DataBlockDeltaEvent:
		db := m.findDataBlock(e.BlockIndex)
		if db == nil {
			m.Content = append(m.Content, NewDataBlock(TypeData, &Source{MediaType: e.MediaType}))
			db = m.findDataBlock(e.BlockIndex)
		}
		if db != nil {
			if db.Source == nil {
				db.Source = &Source{MediaType: e.MediaType}
			}
			db.Source.Data += e.Data
		}
	case *event.DataBlockEndEvent:
		// No-op

	// Tool calls
	case *event.ToolCallStartEvent:
		m.Content = append(m.Content, NewToolUseBlock(e.ToolCallID, e.ToolName, nil))
	case *event.ToolCallDeltaEvent:
		if tu := m.findToolUseBlock(e.ToolCallID); tu != nil {
			tu.RawInput += e.Delta
			var input map[string]any
			if err := json.Unmarshal([]byte(tu.RawInput), &input); err == nil {
				tu.Input = input
			}
		}
	case *event.ToolCallEndEvent:
		// No-op

	// Tool results
	case *event.ToolResultStartEvent:
		tr := NewToolResultBlock(e.ToolCallID, nil, false)
		tr.State = "running"
		m.Content = append(m.Content, tr)
	case *event.ToolResultTextDeltaEvent:
		tr := m.findToolResultBlock(e.ToolCallID)
		if tr == nil {
			tr = NewToolResultBlock(e.ToolCallID, nil, false)
			tr.State = "running"
			m.Content = append(m.Content, tr)
		}
		tr.Content = append(tr.Content, NewTextBlock(e.Delta))
	case *event.ToolResultDataDeltaEvent:
		tr := m.findToolResultBlock(e.ToolCallID)
		if tr == nil {
			tr = NewToolResultBlock(e.ToolCallID, nil, false)
			tr.State = "running"
			m.Content = append(m.Content, tr)
		}
		if block := dataDeltaToBlock(e.Data, e.MediaType); block != nil {
			tr.Content = append(tr.Content, block)
		}
	case *event.ToolResultEndEvent:
		if tr := m.findToolResultBlock(e.ToolCallID); tr != nil {
			tr.State = "completed"
		}

	// Terminal / error events (no content changes)
	case *event.ExceedMaxItersEvent, *event.ErrorEvent, *event.InterruptEvent:
		// No-op
	}

	return m
}

// ---------------------------------------------------------------------------
// Block finders (by per-type index or unique ID)
// ---------------------------------------------------------------------------

func (m *Msg) findTextBlock(idx int) *TextBlock {
	count := 0
	for _, b := range m.Content {
		if tb, ok := b.(*TextBlock); ok {
			if count == idx {
				return tb
			}
			count++
		}
	}
	return nil
}

func (m *Msg) findThinkingBlock(idx int) *ThinkingBlock {
	count := 0
	for _, b := range m.Content {
		if tb, ok := b.(*ThinkingBlock); ok {
			if count == idx {
				return tb
			}
			count++
		}
	}
	return nil
}

func (m *Msg) findHintBlock(idx int) *HintBlock {
	count := 0
	for _, b := range m.Content {
		if hb, ok := b.(*HintBlock); ok {
			if count == idx {
				return hb
			}
			count++
		}
	}
	return nil
}

func (m *Msg) findDataBlock(idx int) *DataBlock {
	count := 0
	for _, b := range m.Content {
		if db, ok := b.(*DataBlock); ok {
			if count == idx {
				return db
			}
			count++
		}
	}
	return nil
}

func (m *Msg) findToolUseBlock(id string) *ToolUseBlock {
	for _, b := range m.Content {
		if tu, ok := b.(*ToolUseBlock); ok && tu.ID == id {
			return tu
		}
	}
	return nil
}

func (m *Msg) findToolResultBlock(id string) *ToolResultBlock {
	for _, b := range m.Content {
		if tr, ok := b.(*ToolResultBlock); ok && tr.ToolUseID == id {
			return tr
		}
	}
	return nil
}

// dataDeltaToBlock converts a raw data delta string and media type into the
// appropriate ContentBlock. Used for ToolResultDataDeltaEvent reconstruction.
func dataDeltaToBlock(data, mediaType string) ContentBlock {
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return NewImageBlock(data, "", mediaType)
	case strings.HasPrefix(mediaType, "audio/"):
		return NewAudioBlock(data, "", mediaType)
	case strings.HasPrefix(mediaType, "video/"):
		return NewVideoBlock(data)
	default:
		src := &Source{MediaType: mediaType, Data: data}
		return NewDataBlock(TypeData, src)
	}
}
