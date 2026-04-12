package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	chatModel, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("Assistant").
		SysPrompt("You are a helpful AI assistant.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	response, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Hello! What can you help me with?").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Assistant: %s\n", response.GetTextContent())
}
