package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
	"github.com/linkerlin/agentscope.go/msghub"
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

	coder, _ := react.Builder().
		Name("Coder").
		SysPrompt("You are a senior Go engineer. Provide a concise code snippet.").
		Model(chatModel).
		Build()

	reviewer, _ := react.Builder().
		Name("Reviewer").
		SysPrompt("You are a code reviewer. Give one brief improvement tip.").
		Model(chatModel).
		Build()

	hub := msghub.New()
	hub.Register("coder", coder)
	hub.Register("reviewer", reviewer)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("Write a goroutine pool in Go.").Build()
	results := hub.Broadcast(context.Background(), msg)

	for name, resp := range results {
		fmt.Printf("=== %s ===\n%s\n\n", name, resp.GetTextContent())
	}
}
