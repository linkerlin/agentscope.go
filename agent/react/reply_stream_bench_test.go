package react

import (
	"context"
	"testing"
	"time"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// benchChatModel is a configurable mock for benchmarks.
type benchChatModel struct {
	name      string
	respText  string
	toolCalls []message.ContentBlock
}

func (m *benchChatModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	msg := message.NewMsg().Role(message.RoleAssistant).TextContent(m.respText).Build()
	if len(m.toolCalls) > 0 {
		msg.Content = append(msg.Content, m.toolCalls...)
	}
	return msg, nil
}

func (m *benchChatModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: m.respText}
	ch <- &model.StreamChunk{Done: true}
	return ch, nil
}

func (m *benchChatModel) ModelName() string { return m.name }

// BenchmarkReplyStream_TextOnly measures the latency of a simple text-only
// reply stream.
func BenchmarkReplyStream_TextOnly(b *testing.B) {
	m := &benchChatModel{name: "mock", respText: "hello world"}
	mem := memory.NewInMemoryMemory()
	agent, err := Builder().Name("bench").Model(m).Memory(mem).Build()
	if err != nil {
		b.Fatal(err)
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ch, _ := agent.ReplyStream(ctx, msg)
		for range ch {
		}
		cancel()
	}
}

// BenchmarkReplyStream_WithTools measures reply stream latency when the agent
// has tools registered (non-streaming model path due to requestTools=true).
func BenchmarkReplyStream_WithTools(b *testing.B) {
	m := &benchChatModel{
		name:     "mock",
		respText: "result",
		toolCalls: []message.ContentBlock{
			message.NewToolUseBlock("t1", "echo", map[string]any{"text": "hello"}),
		},
	}
	mem := memory.NewInMemoryMemory()
	agent, err := Builder().
		Name("bench").
		Model(m).
		Memory(mem).
		Tools(&mockTool{name: "echo", result: "ok"}).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	msg := message.NewMsg().Role(message.RoleUser).TextContent("hi").Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ch, _ := agent.ReplyStream(ctx, msg)
		for range ch {
		}
		cancel()
	}
}
