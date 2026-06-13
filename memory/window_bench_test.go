package memory

import (
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func BenchmarkWindowMemoryAdd(b *testing.B) {
	m := NewWindowMemory(WindowOptions{MaxMessages: 100, MaxTokens: 5000, Tokenizer: RuneTokenizer{}})
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hello world").Build()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Add(msg)
	}
}
