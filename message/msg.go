package message

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Msg represents a message in the agent conversation
type Msg struct {
	ID       string
	Role     MsgRole
	Name     string
	Content  []ContentBlock
	Metadata map[string]any
	CreateAt time.Time
}

// GetTextContent concatenates all text blocks into a single string
func (m *Msg) GetTextContent() string {
	var sb strings.Builder
	for _, block := range m.Content {
		if tb, ok := block.(*TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	return sb.String()
}

// GetToolUseCalls returns all ToolUseBlock entries in the message
func (m *Msg) GetToolUseCalls() []*ToolUseBlock {
	var result []*ToolUseBlock
	for _, block := range m.Content {
		if tb, ok := block.(*ToolUseBlock); ok {
			result = append(result, tb)
		}
	}
	return result
}

// GetToolResults returns all ToolResultBlock entries in the message
func (m *Msg) GetToolResults() []*ToolResultBlock {
	var result []*ToolResultBlock
	for _, block := range m.Content {
		if tb, ok := block.(*ToolResultBlock); ok {
			result = append(result, tb)
		}
	}
	return result
}

// MsgBuilder provides a fluent API for constructing Msg instances
type MsgBuilder struct {
	msg *Msg
}

// NewMsg returns a new MsgBuilder
func NewMsg() *MsgBuilder {
	return &MsgBuilder{
		msg: &Msg{
			ID:       uuid.New().String(),
			Metadata: make(map[string]any),
			CreateAt: time.Now(),
		},
	}
}

func (b *MsgBuilder) Role(role MsgRole) *MsgBuilder {
	b.msg.Role = role
	return b
}

func (b *MsgBuilder) Name(name string) *MsgBuilder {
	b.msg.Name = name
	return b
}

func (b *MsgBuilder) TextContent(text string) *MsgBuilder {
	b.msg.Content = append(b.msg.Content, NewTextBlock(text))
	return b
}

func (b *MsgBuilder) Content(blocks ...ContentBlock) *MsgBuilder {
	b.msg.Content = append(b.msg.Content, blocks...)
	return b
}

func (b *MsgBuilder) Metadata(key string, value any) *MsgBuilder {
	b.msg.Metadata[key] = value
	return b
}

func (b *MsgBuilder) Build() *Msg {
	return b.msg
}
