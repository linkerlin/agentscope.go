package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/hook/recorder"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model"
)

// mockModel is a simple in-memory ChatModel for demonstration purposes.
type mockModel struct{}

func (m *mockModel) Chat(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (*message.Msg, error) {
	return message.NewMsg().
		Role(message.RoleAssistant).
		TextContent("Hello from mock model!").
		Build(), nil
}

func (m *mockModel) ChatStream(ctx context.Context, messages []*message.Msg, options ...model.ChatOption) (<-chan *model.StreamChunk, error) {
	ch := make(chan *model.StreamChunk, 2)
	ch <- &model.StreamChunk{Delta: "Hello from mock model!"}
	ch <- &model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockModel) ModelName() string { return "mock" }

func main() {
	tracePath := filepath.Join(os.TempDir(), "agentscope-trace.jsonl")

	exporter, err := recorder.NewBuilder(tracePath).
		IncludeReasoningChunks(true).
		IncludeActingChunks(true).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	defer exporter.Close()

	chatModel := &mockModel{}

	agent, err := react.Builder().
		Name("TraceDemoAgent").
		SysPrompt("You are a helpful assistant.").
		Model(chatModel).
		StreamHooks(exporter).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	for i, q := range []string{"Hello!", "What can you do?"} {
		fmt.Printf("Turn %d - User: %s\n", i+1, q)
		resp, err := agent.Call(context.Background(), message.NewMsg().
			Role(message.RoleUser).
			TextContent(q).
			Build())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Turn %d - Agent: %s\n\n", i+1, resp.GetTextContent())
	}

	// Ensure exporter flushes before reading
	if err := exporter.Close(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("--- Trace output ---")
	f, err := os.Open(tracePath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 1
	for scanner.Scan() {
		fmt.Printf("%d: %s\n", lineNo, scanner.Text())
		lineNo++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nTrace file written to: %s\n", tracePath)
}
