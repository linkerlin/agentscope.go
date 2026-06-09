package model

const defaultContextSize = 128000

// ContextSized is implemented by ChatModel providers that expose context window size.
type ContextSized interface {
	ContextSize() int
}

// ResolveContextSize returns explicit size, model-provided size, or the library default.
func ResolveContextSize(m ChatModel, explicit int) int {
	if explicit > 0 {
		return explicit
	}
	if cs, ok := m.(ContextSized); ok {
		if n := cs.ContextSize(); n > 0 {
			return n
		}
	}
	return defaultContextSize
}
