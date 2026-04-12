package memory

import (
	"fmt"
	"testing"

	"github.com/linkerlin/agentscope.go/message"
)

func BenchmarkSimpleTokenCounterCountMessages(b *testing.B) {
	c := NewSimpleTokenCounter()
	msgs := []*message.Msg{
		message.NewMsg().Role(message.RoleUser).TextContent("hello world").Build(),
		message.NewMsg().Role(message.RoleAssistant).TextContent("response text").Build(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.CountMessages(msgs)
	}
}

func BenchmarkLocalVectorStoreSearch(b *testing.B) {
	e := fixedEmbed{dim: 4}
	s := NewLocalVectorStore(e)
	ctx := b.Context()
	for i := 0; i < 50; i++ {
		n := NewMemoryNode(MemoryTypePersonal, "u", fmt.Sprintf("doc content %d", i))
		_ = s.Insert(ctx, []*MemoryNode{n})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Search(ctx, "content", RetrieveOptions{TopK: 10, MinScore: 0})
	}
}
