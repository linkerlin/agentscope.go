package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/internal/llmenv"
	"github.com/linkerlin/agentscope.go/message"
)

func main() {
	chatModel := llmenv.MustChatModel()

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
