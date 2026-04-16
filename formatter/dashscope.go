package formatter

// DashScopeFormatter is an alias for OpenAIFormatter because DashScope
// provides an OpenAI-compatible API.
type DashScopeFormatter = OpenAIFormatter

// NewDashScopeFormatter creates a new DashScopeFormatter.
func NewDashScopeFormatter() *DashScopeFormatter {
	return NewOpenAIFormatter()
}
