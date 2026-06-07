package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/linkerlin/agentscope.go/message"
)

// MsgToStored converts a *message.Msg into a *StoredMessage for persistence.
// The sessionID parameter identifies which session the message belongs to.
func MsgToStored(m *message.Msg, sessionID string) *StoredMessage {
	content := m.GetTextContent()
	var blocks string
	if len(m.Content) > 0 {
		// Leverage Msg.MarshalJSON to obtain properly typed content blocks,
		// then extract the raw content array.
		data, _ := json.Marshal(m)
		var raw struct {
			Content []json.RawMessage `json:"content"`
		}
		_ = json.Unmarshal(data, &raw)
		if len(raw.Content) > 0 {
			b, _ := json.Marshal(raw.Content)
			blocks = string(b)
		}
	}
	return &StoredMessage{
		ID:         m.ID,
		SessionID:  sessionID,
		Role:       string(m.Role),
		Name:       m.Name,
		Content:    content,
		Metadata:   m.Metadata,
		CreatedAt:  m.CreatedAt,
		FinishedAt: m.FinishedAt,
		Blocks:     blocks,
		Usage:      m.Usage,
	}
}

// StoredToMsg reconstructs a *message.Msg from a *StoredMessage.
// It restores full content blocks when Blocks is present; otherwise it falls
// back to a single TextBlock built from Content (backward compatibility).
func StoredToMsg(s *StoredMessage) (*message.Msg, error) {
	raw := map[string]any{
		"id":         s.ID,
		"role":       s.Role,
		"name":       s.Name,
		"metadata":   s.Metadata,
		"created_at": s.CreatedAt.Format(time.RFC3339Nano),
	}
	if s.FinishedAt != nil {
		raw["finished_at"] = s.FinishedAt.Format(time.RFC3339Nano)
	}
	if s.Blocks != "" {
		var blocks []any
		if err := json.Unmarshal([]byte(s.Blocks), &blocks); err != nil {
			return nil, fmt.Errorf("unmarshal blocks: %w", err)
		}
		raw["content"] = blocks
	} else if s.Content != "" {
		raw["content"] = []any{map[string]any{"type": "text", "text": s.Content}}
	} else {
		raw["content"] = []any{}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal reconstructed msg: %w", err)
	}
	var msg message.Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal to Msg: %w", err)
	}
	msg.Usage = s.Usage
	return &msg, nil
}
