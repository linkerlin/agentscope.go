//go:build ignore

// Example: langsmith demonstrates streaming Agent events to LangSmith.
//
// Usage:
//
//	export LANGSMITH_API_KEY=ls-...
//	go run examples/langsmith/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
	"github.com/linkerlin/agentscope.go/observability"
)

// mockModel is a simple in-memory ChatModel for demonstration purposes.
type mockModel struct{}

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().
		Role(message.RoleAssistant).
		TextContent("Hello from LangSmith demo!").
		Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 3)
	ch <- &model.StreamChunk{Delta: "Hello "}
	ch <- &model.StreamChunk{Delta: "from LangSmith demo!"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockModel) ModelName() string { return "mock" }

func main() {
	apiKey := os.Getenv("LANGSMITH_API_KEY")
	if apiKey == "" {
		log.Println("LANGSMITH_API_KEY not set; running in dry-run mode (no actual upload)")
		apiKey = "dry-run-key"
	}

	client := observability.NewLangSmithClient(apiKey)
	observer := observability.NewLangSmithObserver(client, "agentscope-go-demo", "session-1")

	bus := event.NewBus(100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go observer.Observe(ctx, bus)

	chatModel := &mockModel{}
	agent, err := react.Builder().
		Name("LangSmithDemoAgent").
		SysPrompt("You are a helpful assistant.").
		Model(chatModel).
		WithEventBus(bus).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Say hello!").
		Build())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Agent:", resp.GetTextContent())
	fmt.Println("Done. Check LangSmith dashboard for traces.")
}
