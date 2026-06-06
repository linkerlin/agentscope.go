package message

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// rawMsg is a JSON-serializable representation of Msg.
type rawMsg struct {
	ID        string         `json:"id"`
	Role      MsgRole        `json:"role"`
	Name      string         `json:"name"`
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
// It is aligned with Python agentscope v2 Pydantic models for cross-language
// compatibility.
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
	Input      any            `json:"input,omitempty"`     // string (PyV2) or map[string]any (legacy)
	RawInput   string         `json:"raw_input,omitempty"` // legacy Go field
	ToolUseID  string         `json:"tool_use_id,omitempty"`
	IsError    bool           `json:"is_error,omitempty"`
	State      string         `json:"state,omitempty"`
	Thinking   string         `json:"thinking,omitempty"`
	Signature  string         `json:"signature,omitempty"`
	Hint       string         `json:"hint,omitempty"`
	HintKind   string         `json:"hint_kind,omitempty"`
	SubContent []rawBlock     `json:"sub_content,omitempty"`
	Output     []rawBlock     `json:"output,omitempty"`     // PyV2 compatibility
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
		src := sourceToRaw(v.Source)
		if src != nil && src.MediaType == "" {
			switch v.BlockType_ {
			case TypeImage:
				src.MediaType = "image/png"
			case TypeAudio:
				src.MediaType = "audio/mpeg"
			case TypeVideo:
				src.MediaType = "video/mp4"
			}
		}
		return rawBlock{Type: TypeData, Source: src}
	case *ImageBlock:
		mime := v.MimeType
		if mime == "" {
			mime = "image/png"
		}
		var src *rawSource
		if v.URL != "" {
			src = &rawSource{Type: SourceTypeURL, MediaType: mime, URL: v.URL}
		} else if v.Base64 != "" {
			src = &rawSource{Type: SourceTypeBase64, MediaType: mime, Data: v.Base64}
		}
		return rawBlock{Type: TypeData, Source: src}
	case *AudioBlock:
		mime := v.MimeType
		if mime == "" {
			mime = "audio/mpeg"
		}
		var src *rawSource
		if v.URL != "" {
			src = &rawSource{Type: SourceTypeURL, MediaType: mime, URL: v.URL}
		} else if v.Base64 != "" {
			src = &rawSource{Type: SourceTypeBase64, MediaType: mime, Data: v.Base64}
		}
		return rawBlock{Type: TypeData, Source: src}
	case *VideoBlock:
		var src *rawSource
		if v.URL != "" {
			src = &rawSource{Type: SourceTypeURL, MediaType: "video/mp4", URL: v.URL}
		}
		return rawBlock{Type: TypeData, Source: src}
	case *ToolUseBlock:
		inputStr := v.RawInput
		if inputStr == "" && v.Input != nil {
			b, _ := json.Marshal(v.Input)
			inputStr = string(b)
		}
		return rawBlock{Type: TypeToolCall, ID: v.ID, Name: v.Name, Input: inputStr, RawInput: v.RawInput}
	case *ToolResultBlock:
		var subs []rawBlock
		for _, sb := range v.Content {
			subs = append(subs, blockToRaw(sb))
		}
		// Cross-lang: Python ToolResultBlock uses 'id' for the tool_call_id.
		id := v.ToolUseID
		if id == "" {
			id = v.ID
		}
		name := v.Name
		if name == "" {
			name = id
		}
		return rawBlock{Type: TypeToolResult, ID: id, Name: name, ToolUseID: v.ToolUseID, IsError: v.IsError, State: v.State, SubContent: subs, Output: subs}
	case *ThinkingBlock:
		return rawBlock{Type: TypeThinking, Thinking: v.Thinking, Signature: v.Signature}
	case *HintBlock:
		return rawBlock{Type: TypeHint, Hint: v.Text, HintKind: v.Kind}
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
	case TypeData:
		if r.Source == nil {
			return nil, fmt.Errorf("message: data block missing source")
		}
		mt := r.Source.MediaType
		switch {
		case strings.HasPrefix(mt, "image/"):
			return NewImageBlock(r.Source.URL, r.Source.Data, mt), nil
		case strings.HasPrefix(mt, "audio/"):
			return NewAudioBlock(r.Source.URL, r.Source.Data, mt), nil
		case strings.HasPrefix(mt, "video/"):
			return NewVideoBlock(r.Source.URL), nil
		default:
			return NewDataBlock(TypeData, sourceFromRaw(r.Source)), nil
		}
	case TypeToolUse, TypeToolCall:
		name := r.Name
		if name == "" {
			name = r.ToolName
		}
		var input map[string]any
		var rawInput string
		switch v := r.Input.(type) {
		case map[string]any:
			input = v
		case string:
			rawInput = v
			_ = json.Unmarshal([]byte(v), &input)
		}
		if rawInput == "" {
			rawInput = r.RawInput
		}
		b := NewToolUseBlock(r.ID, name, input)
		b.RawInput = rawInput
		return b, nil
	case TypeToolResult:
		rawSubs := r.SubContent
		if len(rawSubs) == 0 {
			rawSubs = r.Output
		}
		var subs []ContentBlock
		for _, s := range rawSubs {
			b, err := rawToBlock(s)
			if err != nil {
				return nil, err
			}
			subs = append(subs, b)
		}
		toolUseID := r.ID
		if toolUseID == "" {
			toolUseID = r.ToolUseID
		}
		b := NewToolResultBlock(toolUseID, subs, r.IsError)
		b.ID = r.ID
		b.Name = r.Name
		b.State = r.State
		return b, nil
	case TypeThinking:
		return NewThinkingBlock(r.Thinking, r.Signature), nil
	case TypeHint:
		return NewHintBlock(r.Hint, r.HintKind), nil
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
