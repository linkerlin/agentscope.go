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

// rawSource mirrors Source for JSON serialization.
type rawSource struct {
	Type      SourceType `json:"type"`
	MediaType string     `json:"media_type,omitempty"`
	Data      string     `json:"data,omitempty"`
	URL       string     `json:"url,omitempty"`
}

// rawBlock is a JSON-serializable representation of ContentBlock.
type rawBlock struct {
	Type       BlockType      `json:"type"`
	Text       string         `json:"text,omitempty"`
	URL        string         `json:"url,omitempty"`
	Base64     string         `json:"base64,omitempty"`
	MimeType   string         `json:"mime_type,omitempty"`
	Source     *rawSource     `json:"source,omitempty"`
	ID         string         `json:"id,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
	RawInput   string         `json:"raw_input,omitempty"`
	ToolUseID  string         `json:"tool_use_id,omitempty"`
	IsError    bool           `json:"is_error,omitempty"`
	State      string         `json:"state,omitempty"`
	Thinking   string         `json:"thinking,omitempty"`
	Signature  string         `json:"signature,omitempty"`
	SubContent []rawBlock     `json:"sub_content,omitempty"`
}

func sourceToRaw(s *Source) *rawSource {
	if s == nil {
		return nil
	}
	return &rawSource{
		Type:      s.Type,
		MediaType: s.MediaType,
		Data:      s.Data,
		URL:       s.URL,
	}
}

func sourceFromRaw(r *rawSource) *Source {
	if r == nil {
		return nil
	}
	return &Source{
		Type:      r.Type,
		MediaType: r.MediaType,
		Data:      r.Data,
		URL:       r.URL,
	}
}

func blockToRaw(b ContentBlock) rawBlock {
	switch v := b.(type) {
	case *TextBlock:
		return rawBlock{Type: TypeText, Text: v.Text}
	case *DataBlock:
		return rawBlock{Type: v.BlockType_, Source: sourceToRaw(v.Source)}
	case *ImageBlock:
		var src *rawSource
		if v.URL != "" {
			src = &rawSource{Type: SourceTypeURL, URL: v.URL}
		} else if v.Base64 != "" {
			src = &rawSource{Type: SourceTypeBase64, MediaType: v.MimeType, Data: v.Base64}
		}
		return rawBlock{Type: TypeImage, Source: src}
	case *AudioBlock:
		var src *rawSource
		if v.URL != "" {
			src = &rawSource{Type: SourceTypeURL, URL: v.URL}
		} else if v.Base64 != "" {
			src = &rawSource{Type: SourceTypeBase64, MediaType: v.MimeType, Data: v.Base64}
		}
		return rawBlock{Type: TypeAudio, Source: src}
	case *VideoBlock:
		var src *rawSource
		if v.URL != "" {
			src = &rawSource{Type: SourceTypeURL, URL: v.URL}
		}
		return rawBlock{Type: TypeVideo, Source: src}
	case *ToolUseBlock:
		return rawBlock{Type: TypeToolUse, ID: v.ID, ToolName: v.Name, Input: v.Input, RawInput: v.RawInput}
	case *ToolResultBlock:
		var subs []rawBlock
		for _, sb := range v.Content {
			subs = append(subs, blockToRaw(sb))
		}
		return rawBlock{Type: TypeToolResult, ID: v.ID, Name: v.Name, ToolUseID: v.ToolUseID, IsError: v.IsError, State: v.State, SubContent: subs}
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
		if r.Source != nil {
			return NewImageBlock(r.Source.URL, r.Source.Data, r.Source.MediaType), nil
		}
		return NewImageBlock(r.URL, r.Base64, r.MimeType), nil
	case TypeAudio:
		if r.Source != nil {
			return NewAudioBlock(r.Source.URL, r.Source.Data, r.Source.MediaType), nil
		}
		return NewAudioBlock(r.URL, r.Base64, r.MimeType), nil
	case TypeVideo:
		if r.Source != nil {
			return NewVideoBlock(r.Source.URL), nil
		}
		return NewVideoBlock(r.URL), nil
	case TypeToolUse:
		b := NewToolUseBlock(r.ID, r.ToolName, r.Input)
		b.RawInput = r.RawInput
		return b, nil
	case TypeToolResult:
		var subs []ContentBlock
		for _, s := range r.SubContent {
			b, err := rawToBlock(s)
			if err != nil {
				return nil, err
			}
			subs = append(subs, b)
		}
		b := NewToolResultBlock(r.ToolUseID, subs, r.IsError)
		b.ID = r.ID
		b.Name = r.Name
		b.State = r.State
		return b, nil
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
