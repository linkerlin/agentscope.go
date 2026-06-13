// anthropic_stream demonstrates consuming the V2 event stream via Anthropic Claude.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/anthropic"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}

	model, err := anthropic.NewBuilder().
		APIKey(apiKey).
		ModelName("claude-3-5-sonnet-20241022").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("StreamingClaude").
		SysPrompt("You are a helpful assistant powered by Claude.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	ch, err := agent.ReplyStream(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Count from 1 to 5 and explain why Go channels are useful.").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Streaming Response ===")
	for ev := range ch {
		switch e := ev.(type) {
		case *event.TextBlockDeltaEvent:
			fmt.Print(e.Delta)
		case *event.TextBlockEndEvent:
			fmt.Println()
		case *event.ToolCallStartEvent:
			fmt.Printf("\n[Tool Call] %s\n", e.ToolName)
		case *event.ReplyEndEvent:
			fmt.Println("\n[Reply End]")
		}
	}
}
