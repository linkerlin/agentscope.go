package model

import (
	"context"

	"github.com/linkerlin/agentscope.go/message"
)

// ChatModel is the interface that all LLM model implementations must satisfy
type ChatModel interface {
	Chat(ctx context.Context, messages []*message.Msg, options ...ChatOption) (*message.Msg, error)
	ChatStream(ctx context.Context, messages []*message.Msg, options ...ChatOption) (<-chan *StreamChunk, error)
	ModelName() string
}

// ChatOptions holds configuration for a model call
type ChatOptions struct {
	MaxTokens   int
	Temperature float64
	Tools       []ToolSpec
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

// StreamChunk is a single chunk from a streaming model response
type StreamChunk struct {
	Delta   string
	Content []message.ContentBlock
	Done    bool
}
