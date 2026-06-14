package a2atool

import (
	"context"
	"fmt"
	"testing"

	"github.com/linkerlin/agentscope.go/a2a"
)

type benchMockClient struct{}

func (m *benchMockClient) Send(ctx context.Context, msg *a2a.Message) (*a2a.Message, error) {
	return &a2a.Message{Role: "agent", Content: "ok"}, nil
}

func (m *benchMockClient) SendSubscribe(ctx context.Context, msg *a2a.Message) (<-chan *a2a.Message, error) {
	ch := make(chan *a2a.Message, 1)
	ch <- &a2a.Message{Content: "ok"}
	close(ch)
	return ch, nil
}

func (m *benchMockClient) Close() error { return nil }

func BenchmarkA2ATool_ExecuteSync(b *testing.B) {
	tool := NewA2ATool("remote", "A remote agent", &benchMockClient{})
	input := map[string]any{"task": "Do something"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(context.Background(), input)
	}
}

func BenchmarkA2ATool_ExecuteStreaming(b *testing.B) {
	tool := NewA2ATool("remote", "A remote agent", &benchMockClient{}).WithStreaming(true)
	input := map[string]any{"task": "Stream something"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(context.Background(), input)
	}
}

func BenchmarkRegistry_AllTools(b *testing.B) {
	r := NewRegistry(func(url string) a2a.Client { return &benchMockClient{} })
	for i := 0; i < 10; i++ {
		r.Register(fmt.Sprintf("agent%d", i), "desc", fmt.Sprintf("http://agent%d:8080", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.AllTools()
	}
}
