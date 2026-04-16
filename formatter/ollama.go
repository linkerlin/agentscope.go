package formatter

// OllamaFormatter is an alias for OpenAIFormatter because Ollama
// provides an OpenAI-compatible API.
type OllamaFormatter = OpenAIFormatter

// NewOllamaFormatter creates a new OllamaFormatter.
func NewOllamaFormatter() *OllamaFormatter {
	return NewOpenAIFormatter()
}
