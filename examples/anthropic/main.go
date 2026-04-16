package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/anthropic"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set ANTHROPIC_API_KEY")
		return
	}

	chatModel, err := anthropic.NewBuilder().
		APIKey(apiKey).
		ModelName("claude-3-5-sonnet-20241022").
		Build()
	if err != nil {
		panic(err)
	}

	agent, err := react.Builder().
		Name("ClaudeAssistant").
		SysPrompt("You are a helpful assistant powered by Claude.").
		Model(chatModel).
		Build()
	if err != nil {
		panic(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Explain Go interfaces in one sentence.").
		Build())
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.GetTextContent())
}
