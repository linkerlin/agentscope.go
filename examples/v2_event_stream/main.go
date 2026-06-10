package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/event"
	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	model, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("Assistant").
		SysPrompt("You are a helpful assistant. Keep responses concise.").
		Model(model).
		Memory(memory.NewInMemoryMemory()).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	msg := message.NewMsg().
		Role(message.RoleUser).
		TextContent("Explain Go channels in exactly 2 sentences.").
		Build()

	fmt.Println("=== V2 Event Stream Demo ===")
	fmt.Println()

	ch, err := agent.ReplyStream(ctx, msg)
	if err != nil {
		log.Fatal(err)
	}

	var thinking string
	var output string

	for ev := range ch {
		switch e := ev.(type) {
		case *event.ReplyStartEvent:
			fmt.Printf("[START] reply_id=%s\n", e.ReplyID())

		case *event.ThinkingBlockStartEvent:
			fmt.Printf("[THINK_START] block=%d\n", e.BlockIndex)
		case *event.ThinkingBlockDeltaEvent:
			thinking += e.Delta
		case *event.ThinkingBlockEndEvent:
			if thinking != "" {
				fmt.Printf("  (thought for %d chars)\n", len(thinking))
			}
			thinking = ""

		case *event.TextBlockStartEvent:
			fmt.Printf("[TEXT_START] block=%d\n", e.BlockIndex)
		case *event.TextBlockDeltaEvent:
			fmt.Print(e.Delta)
			output += e.Delta
		case *event.TextBlockEndEvent:
			fmt.Println()

		case *event.ToolCallStartEvent:
			fmt.Printf("[TOOL_START] tool=%s call_id=%s\n", e.ToolName, e.ToolCallID)
		case *event.ToolCallDeltaEvent:
			fmt.Printf("[TOOL_ARGS] %s\n", e.Delta)
		case *event.ToolCallEndEvent:
			fmt.Printf("[TOOL_END] call_id=%s\n", e.ToolCallID)

		case *event.ToolResultStartEvent:
			fmt.Printf("[TOOL_RESULT_START] call_id=%s\n", e.ToolCallID)
		case *event.ToolResultTextDeltaEvent:
			fmt.Printf("  [result] %s\n", e.Delta)
		case *event.ToolResultEndEvent:
			fmt.Printf("[TOOL_RESULT_END]\n")

		case *event.ErrorEvent:
			fmt.Printf("[ERROR] %v\n", e.Err)

		case *event.ReplyEndEvent:
			fmt.Printf("\n[END] reply_id=%s\n", e.ReplyID())
		default:
			fmt.Printf("[EVENT] %s (reply_id=%s)\n", ev.EventType(), ev.ReplyID())
		}
	}

	fmt.Printf("\n=== Final Output ===\n%s\n", output)
}
