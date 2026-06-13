package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/deepseek"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable is required")
	}

	ctx := context.Background()

	// Create model using Builder pattern
	model, err := deepseek.Builder(apiKey).Build()
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	agent1, err := react.Builder().
		Name("Agent-A").
		Model(model).
		SysPrompt("You are a helpful assistant named Agent-A.").
		Build()
	if err != nil {
		log.Fatalf("Failed to create Agent-A: %v", err)
	}

	agent2, err := react.Builder().
		Name("Agent-B").
		Model(model).
		SysPrompt("You are a helpful assistant named Agent-B.").
		Build()
	if err != nil {
		log.Fatalf("Failed to create Agent-B: %v", err)
	}

	hub := msghub.New()
	hub.Register("Agent-A", agent1)
	hub.Register("Agent-B", agent2)

	// Send a test message through the hub
	testMsg := message.NewMsg().Role(message.RoleUser).TextContent("Hello everyone! Please introduce yourselves.").Build()
	responses := hub.Broadcast(ctx, testMsg)
	for name, resp := range responses {
		fmt.Printf("[%s] %s\n", name, resp.GetTextContent())
	}
}
