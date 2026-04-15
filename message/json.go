package message

import (
	"encoding/json"
	"fmt"
	"time"
)

// rawMsg is a JSON-serializable representation of Msg.
type rawMsg struct {
	ID        string         `json:"id"`
	Role      MsgRole        `json:"role"`
	Name      string         `json:"name,omitempty"`
	Content   []rawBlock     `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// rawBlock is a JSON-serializable representation of ContentBlock.
type rawBlock struct {
	Type        BlockType      `json:"type"`
	Text        string         `json:"text,omitempty"`
	URL         string         `json:"url,omitempty"`
	Base64      string         `json:"base64,omitempty"`
	MimeType    string         `json:"mime_type,omitempty"`
	ID          string         `json:"id,omitempty"`
	ToolName    string         `json:"tool_name,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	ToolUseID   string         `json:"tool_use_id,omitempty"`
	IsError     bool           `json:"is_error,omitempty"`
	Thinking    string         `json:"thinking,omitempty"`
	Signature   string         `json:"signature,omitempty"`
	SubContent  []rawBlock     `json:"sub_content,omitempty"`
}

func blockToRaw(b ContentBlock) rawBlock {
	switch v := b.(type) {
	case *TextBlock:
		return rawBlock{Type: TypeText, Text: v.Text}
	case *ImageBlock:
		return rawBlock{Type: TypeImage, URL: v.URL, Base64: v.Base64, MimeType: v.MimeType}
	case *AudioBlock:
		return rawBlock{Type: TypeAudio, URL: v.URL, Base64: v.Base64, MimeType: v.MimeType}
	case *VideoBlock:
		return rawBlock{Type: TypeVideo, URL: v.URL}
	case *ToolUseBlock:
		return rawBlock{Type: TypeToolUse, ID: v.ID, ToolName: v.Name, Input: v.Input}
	case *ToolResultBlock:
		var subs []rawBlock
		for _, sb := range v.Content {
			subs = append(subs, blockToRaw(sb))
		}
		return rawBlock{Type: TypeToolResult, ToolUseID: v.ToolUseID, IsError: v.IsError, SubContent: subs}
	case *ThinkingBlock:
		return rawBlock{Type: TypeThinking, Thinking: v.Thinking, Signature: v.Signature}
	default:
		return rawBlock{Type: TypeText}
	}
}

func rawToBlock(r rawBlock) (ContentBlock, error) {
	switch r.Type {
	case TypeText:
		return NewTextBlock(r.Text), nil
	case TypeImage:
		return NewImageBlock(r.URL, r.Base64, r.MimeType), nil
	case TypeAudio:
		return NewAudioBlock(r.URL, r.Base64, r.MimeType), nil
	case TypeVideo:
		return NewVideoBlock(r.URL), nil
	case TypeToolUse:
		return NewToolUseBlock(r.ID, r.ToolName, r.Input), nil
	case TypeToolResult:
		var subs []ContentBlock
		for _, s := range r.SubContent {
			b, err := rawToBlock(s)
			if err != nil {
				return nil, err
			}
			subs = append(subs, b)
		}
		return NewToolResultBlock(r.ToolUseID, subs, r.IsError), nil
	case TypeThinking:
		return NewThinkingBlock(r.Thinking, r.Signature), nil
	default:
		return nil, fmt.Errorf("message: unknown block type %s", r.Type)
	}
}

// MarshalJSON implements json.Marshaler for Msg.
func (m *Msg) MarshalJSON() ([]byte, error) {
	r := rawMsg{
		ID:        m.ID,
		Role:      m.Role,
		Name:      m.Name,
		Metadata:  m.Metadata,
		CreatedAt: m.CreatedAt,
	}
	for _, b := range m.Content {
		r.Content = append(r.Content, blockToRaw(b))
	}
	return json.Marshal(r)
}

// UnmarshalJSON implements json.Unmarshaler for Msg.
func (m *Msg) UnmarshalJSON(data []byte) error {
	var r rawMsg
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	m.ID = r.ID
	m.Role = r.Role
	m.Name = r.Name
	m.Metadata = r.Metadata
	m.CreatedAt = r.CreatedAt
	for _, rb := range r.Content {
		b, err := rawToBlock(rb)
		if err != nil {
			return err
		}
		m.Content = append(m.Content, b)
	}
	return nil
}
