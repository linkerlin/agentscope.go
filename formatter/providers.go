package formatter

// DeepSeekFormatter reuses OpenAI-compatible message formatting.
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

// MultiAgentOpenAIFormatter wraps OpenAIFormatter for multi-agent conversations.
type MultiAgentOpenAIFormatter struct{ *OpenAIFormatter }

func NewMultiAgentOpenAIFormatter() *MultiAgentOpenAIFormatter {
	return &MultiAgentOpenAIFormatter{OpenAIFormatter: NewOpenAIFormatter()}
}
