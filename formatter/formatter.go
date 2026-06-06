// Package formatter provides model-specific message formatting.
//
// A Formatter converts agent-level Msg and ToolSpec into the native request
// format required by a particular LLM API (OpenAI, Anthropic, Gemini, etc.).
// This decouples message structure from network transport, making it easier to
// add new model backends without duplicating format-conversion logic.
package formatter

import (
	"regexp"
	"strings"

	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// thinkingTagPattern matches common thinking block markers emitted by reasoning
// models (DeepSeek, QwQ, etc.) in their text output.
var thinkingTagPattern = regexp.MustCompile(`(?s)<think>(.*?)</think>|<thinking>(.*?)</thinking>`)

// extractThinkingBlocks scans content for thinking tags, adds ThinkingBlock
// content to the builder, and returns the remaining text.
func extractThinkingBlocks(builder *message.MsgBuilder, content string) string {
	matches := thinkingTagPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var textParts []string
	lastEnd := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start > lastEnd {
			textParts = append(textParts, content[lastEnd:start])
		}
		var thinking string
		if m[2] >= 0 {
			thinking = content[m[2]:m[3]]
		} else if m[4] >= 0 {
			thinking = content[m[4]:m[5]]
		}
		if thinking != "" {
			builder.Content(message.NewThinkingBlock(thinking, ""))
		}
		lastEnd = end
	}
	if lastEnd < len(content) {
		textParts = append(textParts, content[lastEnd:])
	}
	return strings.Join(textParts, "")
}

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

// ThinkingFormatter is an optional extension for formatters that need to wrap
// thinking/reasoning content with provider-specific tags (e.g. Anthropic's
// <thinking> block). If a formatter implements this interface, the agent will
// call it before emitting thinking content.
type ThinkingFormatter interface {
	Formatter
	// WrapThinkingBlock wraps raw thinking content into the model's native
	// representation. For Anthropic this might return `<thinking>content</thinking>`.
	WrapThinkingBlock(content string) string
}
