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
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}

	ctx := context.Background()

	// Create model with structured output support
	m, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o").
		Build()
	if err != nil {
		log.Fatalf("failed to build model: %v", err)
	}

	// Create agent using react.Builder
	agt, err := react.Builder().
		Name("StructuredBot").
		Model(m).
		Build()
	if err != nil {
		log.Fatalf("failed to build agent: %v", err)
	}

	// Send a test message that expects structured output
	msg := message.NewMsg().Role(message.RoleUser).TextContent("List three popular Go web frameworks with their key features in a structured format.").Build()
	resp, err := agt.Reply(ctx, msg)
	if err != nil {
		log.Fatalf("failed to get reply: %v", err)
	}

	fmt.Println("Response:")
	fmt.Println(resp.GetTextContent())
}
