package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set GEMINI_API_KEY")
		return
	}

	chatModel, err := gemini.NewBuilder().
		APIKey(apiKey).
		ModelName("gemini-1.5-flash").
		Build()
	if err != nil {
		panic(err)
	}

	agent, err := react.Builder().
		Name("GeminiAssistant").
		SysPrompt("You are a helpful assistant powered by Gemini.").
		Model(chatModel).
		Build()
	if err != nil {
		panic(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("What is the capital of France?").
		Build())
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.GetTextContent())
}
