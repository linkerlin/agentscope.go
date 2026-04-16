package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/pipeline"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY")
		return
	}

	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		panic(err)
	}

	// Step 1: expand the user's idea into a short outline
	planner, _ := react.Builder().
		Name("Planner").
		SysPrompt("You are a planner. Expand the user's idea into a 3-sentence outline.").
		Model(chatModel).
		Build()

	// Step 2: write a poem based on the outline
	writer, _ := react.Builder().
		Name("Writer").
		SysPrompt("You are a poet. Turn the given outline into a haiku.").
		Model(chatModel).
		Build()

	// Chain them together
	pipe := pipeline.New("CreativePipe", planner, writer)

	resp, err := pipe.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("autumn leaves").
		Build())
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Final Output ===")
	fmt.Println(resp.GetTextContent())
}
