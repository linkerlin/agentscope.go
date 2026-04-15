// Package formatter provides model-specific message formatting.
//
// A Formatter converts agent-level Msg and ToolSpec into the native request
// format required by a particular LLM API (OpenAI, Anthropic, Gemini, etc.).
// This decouples message structure from network transport, making it easier to
// add new model backends without duplicating format-conversion logic.
package formatter

import (
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// Formatter is the generic abstraction for message-to-model formatting.
// Concrete formatters may expose additional typed helpers when the generic
// any-typed methods are inconvenient.
type Formatter interface {
	// FormatMessages converts a slice of agent messages into the model's native message format.
	FormatMessages(msgs []*message.Msg) (any, error)
	// FormatTools converts tool specs into the model's native tool format.
	FormatTools(specs []model.ToolSpec) (any, error)
	// FormatToolChoice converts a tool-choice directive into the model's native representation.
	FormatToolChoice(tc *model.ToolChoice) (any, error)
	// ParseResponse converts a raw model response into a standard *message.Msg.
	ParseResponse(resp any) (*message.Msg, error)
}
