package message

// BlockType identifies the type of content block
type BlockType string

const (
	TypeText       BlockType = "text"
	TypeImage      BlockType = "image"
	TypeAudio      BlockType = "audio"
	TypeVideo      BlockType = "video"
	TypeToolUse    BlockType = "tool_use"
	TypeToolResult BlockType = "tool_result"
	TypeThinking   BlockType = "thinking"
)

// ContentBlock is the interface for all content block types
type ContentBlock interface {
	BlockType() BlockType
}

// TextBlock holds plain text content
type TextBlock struct {
	Text string
}

func NewTextBlock(text string) *TextBlock { return &TextBlock{Text: text} }
func (b *TextBlock) BlockType() BlockType { return TypeText }

// ImageBlock holds image content (URL or base64)
type ImageBlock struct {
	URL      string
	Base64   string
	MimeType string
}

func NewImageBlock(url, base64, mimeType string) *ImageBlock {
	return &ImageBlock{URL: url, Base64: base64, MimeType: mimeType}
}
func (b *ImageBlock) BlockType() BlockType { return TypeImage }

// AudioBlock holds audio content
type AudioBlock struct {
	URL      string
	Base64   string
	MimeType string
}

func NewAudioBlock(url, base64, mimeType string) *AudioBlock {
	return &AudioBlock{URL: url, Base64: base64, MimeType: mimeType}
}
func (b *AudioBlock) BlockType() BlockType { return TypeAudio }

// VideoBlock holds video content
type VideoBlock struct {
	URL string
}

func NewVideoBlock(url string) *VideoBlock { return &VideoBlock{URL: url} }
func (b *VideoBlock) BlockType() BlockType { return TypeVideo }

// ToolUseBlock represents a tool invocation request
type ToolUseBlock struct {
	ID    string
	Name  string
	Input map[string]any
}

func NewToolUseBlock(id, name string, input map[string]any) *ToolUseBlock {
	return &ToolUseBlock{ID: id, Name: name, Input: input}
}
func (b *ToolUseBlock) BlockType() BlockType { return TypeToolUse }

// ToolResultBlock holds the result of a tool invocation
type ToolResultBlock struct {
	ToolUseID string
	Content   []ContentBlock
	IsError   bool
}

func NewToolResultBlock(toolUseID string, content []ContentBlock, isError bool) *ToolResultBlock {
	return &ToolResultBlock{ToolUseID: toolUseID, Content: content, IsError: isError}
}
func (b *ToolResultBlock) BlockType() BlockType { return TypeToolResult }

// ThinkingBlock contains model chain-of-thought reasoning
type ThinkingBlock struct {
	Thinking  string
	Signature string
}

func NewThinkingBlock(thinking, signature string) *ThinkingBlock {
	return &ThinkingBlock{Thinking: thinking, Signature: signature}
}
func (b *ThinkingBlock) BlockType() BlockType { return TypeThinking }
