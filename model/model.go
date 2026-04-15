package model

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// ChatUsage represents token usage reported by an LLM API call.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// Add returns a new ChatUsage with the fields summed.
func (u ChatUsage) Add(other ChatUsage) ChatUsage {
	return ChatUsage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
	}
}

// ChatModel is the interface that all LLM model implementations must satisfy
type ChatModel interface {
	Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error)
	ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error)
	ModelName() string
}

// ToolChoice determines how the model should use tools.
type ToolChoice struct {
	// Mode is one of: "auto", "none", "any", or empty (defaults to auto)
	Mode string
	// Function is required when Mode is not set but a specific tool is desired
	// (for OpenAI-compatible APIs that support {"type":"function","function":{"name":"..."}})
	Function string
}

// ChatOptions holds configuration for a model call
type ChatOptions struct {
	MaxTokens   int
	Temperature float64
	Tools       []ToolSpec
	ToolChoice  *ToolChoice
}

// ToolSpec defines a tool available to the model
type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema object
}

// ChatOption is a functional option for ChatOptions
type ChatOption func(*ChatOptions)

func WithMaxTokens(n int) ChatOption {
	return func(o *ChatOptions) { o.MaxTokens = n }
}

func WithTemperature(t float64) ChatOption {
	return func(o *ChatOptions) { o.Temperature = t }
}

func WithTools(tools []ToolSpec) ChatOption {
	return func(o *ChatOptions) { o.Tools = tools }
}

func WithToolChoice(tc *ToolChoice) ChatOption {
	return func(o *ChatOptions) { o.ToolChoice = tc }
}

// StreamChunk is a single chunk from a streaming model response
type StreamChunk struct {
	Delta   string
	Content []message.ContentBlock
	Done    bool
	Usage   *ChatUsage // optional final usage for the stream
}
