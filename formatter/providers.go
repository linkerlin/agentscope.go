package formatter

import (
	"encoding/json"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/linkerlin/agentscope.go/message"
)

type DeepSeekFormatter struct{ *OpenAIFormatter }

func NewDeepSeekFormatter() *DeepSeekFormatter {
	return &DeepSeekFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

// MoonshotFormatter reuses OpenAI-compatible message formatting (Kimi).
type MoonshotFormatter struct{ *OpenAIFormatter }

func NewMoonshotFormatter() *MoonshotFormatter {
	return &MoonshotFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

// XAIFormatter reuses OpenAI-compatible message formatting (Grok).
type XAIFormatter struct{ *OpenAIFormatter }

func NewXAIFormatter() *XAIFormatter {
	return &XAIFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

// OpenAIResponseFormatter reuses OpenAI-compatible formatting for Response API payloads.
type OpenAIResponseFormatter struct{ *OpenAIFormatter }

func NewOpenAIResponseFormatter() *OpenAIResponseFormatter {
	return &OpenAIResponseFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}

// MultiAgentDeepSeekFormatter formats multi-agent conversations for DeepSeek APIs.
type MultiAgentDeepSeekFormatter struct{ *MultiAgentOpenAIFormatter }

func NewMultiAgentDeepSeekFormatter() *MultiAgentDeepSeekFormatter {
	return &MultiAgentDeepSeekFormatter{MultiAgentOpenAIFormatter: NewMultiAgentOpenAIFormatter()}
}

// MultiAgentMoonshotFormatter formats multi-agent conversations for Moonshot APIs.
type MultiAgentMoonshotFormatter struct{ *MultiAgentOpenAIFormatter }

func NewMultiAgentMoonshotFormatter() *MultiAgentMoonshotFormatter {
	return &MultiAgentMoonshotFormatter{MultiAgentOpenAIFormatter: NewMultiAgentOpenAIFormatter()}
}

// MultiAgentXAIFormatter formats multi-agent conversations for xAI APIs.
type MultiAgentXAIFormatter struct{ *MultiAgentOpenAIFormatter }

func NewMultiAgentXAIFormatter() *MultiAgentXAIFormatter {
	return &MultiAgentXAIFormatter{MultiAgentOpenAIFormatter: NewMultiAgentOpenAIFormatter()}
}

// MultiAgentOpenAIResponseFormatter formats multi-agent conversations for OpenAI Response APIs.
type MultiAgentOpenAIResponseFormatter struct{ *MultiAgentOpenAIFormatter }

func NewMultiAgentOpenAIResponseFormatter() *MultiAgentOpenAIResponseFormatter {
	return &MultiAgentOpenAIResponseFormatter{MultiAgentOpenAIFormatter: NewMultiAgentOpenAIFormatter()}
}

// MultiAgentAnthropicFormatter wraps AnthropicFormatter with multi-agent grouping.
type MultiAgentAnthropicFormatter struct {
	*AnthropicFormatter
}

func NewMultiAgentAnthropicFormatter() *MultiAgentAnthropicFormatter {
	return &MultiAgentAnthropicFormatter{AnthropicFormatter: NewAnthropicFormatter()}
}

func (f *MultiAgentAnthropicFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	typed, _ := f.formatMultiAgent(msgs)
	return typed, nil
}

func (f *MultiAgentAnthropicFormatter) FormatMessagesTyped(msgs []*message.Msg) ([]anthropicMessage, string) {
	return f.formatMultiAgent(msgs)
}

func (f *MultiAgentAnthropicFormatter) formatMultiAgent(msgs []*message.Msg) ([]anthropicMessage, string) {
	if len(msgs) == 0 {
		return nil, ""
	}
	start := 0
	var systemPrompt string
	if msgs[0].Role == message.RoleSystem {
		systemPrompt = msgs[0].GetTextContent()
		start = 1
	}
	var out []anthropicMessage
	firstAgent := true
	for _, g := range GroupMessages(msgs[start:]) {
		switch g.Type {
		case "tool_sequence":
			typed, _ := f.AnthropicFormatter.FormatMessages(g.Msgs)
			out = append(out, typed...)
		case "agent_message":
			openai := formatOpenAIAgentMessageGroup(g.Msgs, firstAgent)
			for _, m := range openai {
				data, _ := json.Marshal([]map[string]any{{"type": "text", "text": m.Content}})
				out = append(out, anthropicMessage{Role: "user", Content: data})
			}
			firstAgent = false
		}
	}
	return out, systemPrompt
}

// MultiAgentGeminiFormatter wraps GeminiFormatter with multi-agent grouping.
type MultiAgentGeminiFormatter struct {
	*GeminiFormatter
}

func NewMultiAgentGeminiFormatter() *MultiAgentGeminiFormatter {
	return &MultiAgentGeminiFormatter{GeminiFormatter: NewGeminiFormatter()}
}

func (f *MultiAgentGeminiFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	contents, _ := f.formatMultiAgentContents(msgs)
	return contents, nil
}

func (f *MultiAgentGeminiFormatter) formatMultiAgentContents(msgs []*message.Msg) ([]map[string]any, string) {
	if len(msgs) == 0 {
		return nil, ""
	}
	start := 0
	var system string
	if msgs[0].Role == message.RoleSystem {
		system = msgs[0].GetTextContent()
		start = 1
	}
	var out []map[string]any
	firstAgent := true
	for _, g := range GroupMessages(msgs[start:]) {
		switch g.Type {
		case "tool_sequence":
			contents, _ := f.GeminiFormatter.FormatContents(g.Msgs)
			out = append(out, contents...)
		case "agent_message":
			openai := formatOpenAIAgentMessageGroup(g.Msgs, firstAgent)
			for _, m := range openai {
				out = append(out, map[string]any{
					"role":  "user",
					"parts": []map[string]any{{"text": m.Content}},
				})
			}
			firstAgent = false
		}
	}
	return out, system
}

// MultiAgentDashScopeFormatter wraps DashScopeFormatter with multi-agent grouping.
type MultiAgentDashScopeFormatter struct {
	*DashScopeFormatter
}

func NewMultiAgentDashScopeFormatter() *MultiAgentDashScopeFormatter {
	return &MultiAgentDashScopeFormatter{DashScopeFormatter: NewDashScopeFormatter()}
}

func (f *MultiAgentDashScopeFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	typed, _ := f.formatMultiAgent(msgs)
	return typed, nil
}

//nolint:unparam
func (f *MultiAgentDashScopeFormatter) formatMultiAgent(msgs []*message.Msg) ([]goopenai.ChatCompletionMessage, error) {
	return FormatOpenAIMultiAgentMessages(NewOpenAIFormatter(), msgs), nil
}

// MultiAgentOllamaFormatter wraps OllamaFormatter with multi-agent grouping.
type MultiAgentOllamaFormatter struct {
	*OllamaFormatter
}

func NewMultiAgentOllamaFormatter() *MultiAgentOllamaFormatter {
	return &MultiAgentOllamaFormatter{OllamaFormatter: NewOllamaFormatter()}
}

func (f *MultiAgentOllamaFormatter) FormatMessages(msgs []*message.Msg) (any, error) {
	typed, _ := f.formatMultiAgent(msgs)
	return typed, nil
}

//nolint:unparam
func (f *MultiAgentOllamaFormatter) formatMultiAgent(msgs []*message.Msg) ([]goopenai.ChatCompletionMessage, error) {
	return FormatOpenAIMultiAgentMessages(NewOpenAIFormatter(), msgs), nil
}
