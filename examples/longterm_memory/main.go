// Example: long-term memory middleware (mirrors Python agentscope Mem0Middleware #1775).
//
// Demonstrates the three modes with an in-memory backend and a mock model:
//   - static_control: memories are retrieved before the reply and injected as
//     a hint, then the new exchange is written back.
//   - agent_control: search_memory / add_memory tools are advertised.
//
// Run: go run ./examples/longterm_memory
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/middleware"
	"github.com/linkerlin/agentscope.go/model"
)

type mockModel struct{}

func (m *mockModel) Chat(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (*message.Msg, error) {
	// Echo any injected memory hint back so we can observe retrieval.
	out := message.NewMsg().Role(message.RoleAssistant)
	for _, msg := range msgs {
		for _, b := range msg.Content {
			if hb, ok := b.(*message.HintBlock); ok {
				out.TextContent("[saw memory hint] " + truncate(hb.Text, 60))
			}
		}
	}
	if out.Build().GetTextContent() == "" {
		out.TextContent("ok")
	}
	return out.Build(), nil
}
func (m *mockModel) ChatStream(ctx context.Context, msgs []*message.Msg, opts ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	resp, _ := m.Chat(ctx, msgs, opts...)
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: resp.GetTextContent()}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}
func (m *mockModel) ModelName() string { return "mock" }

func main() {
	backend := middleware.NewInMemoryLongTermMemory()
	// Seed a durable fact about the user.
	_ = backend.Add(context.Background(), []string{"the user is allergic to peanuts"}, middleware.AddOptions{UserID: "alice"})

	mw := middleware.NewLongTermMemoryMiddleware(backend, "alice").
		WithMode(middleware.MemoryModeStaticControl)

	a, err := react.Builder().
		Name("MemoAgent").
		SysPrompt("You remember facts about the user.").
		Model(&mockModel{}).
		Middlewares(mw).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// First reply: query "peanuts" matches the seeded memory -> injected.
	resp, err := a.Call(context.Background(), message.NewMsg().Role(message.RoleUser).TextContent("peanuts").Build())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("reply:", resp.GetTextContent())

	// The static-control path also wrote the exchange back; snapshot grew.
	snap := backend.Snapshot("alice")
	fmt.Printf("\nmemories stored for alice (%d):\n", len(snap))
	for _, mem := range snap {
		fmt.Println("  -", mem.Text)
	}

	// Agent-control mode advertises tools.
	mwAgent := middleware.NewLongTermMemoryMiddleware(backend, "alice").WithMode(middleware.MemoryModeAgentControl)
	fmt.Println("\nagent_control tools:")
	for _, t := range mwAgent.Tools() {
		fmt.Printf("  - %s: %s\n", t.Name(), t.Description())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
