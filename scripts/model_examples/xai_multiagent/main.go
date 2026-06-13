// xai_multiagent demonstrates a multi-agent conversation using MsgHub via xAI (Grok).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/xai"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		log.Fatal("XAI_API_KEY is required")
	}

	chatModel, err := xai.Builder(apiKey).
		ModelName(xai.ModelGrokBeta).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	planner, err := react.Builder().
		Name("Planner").
		SysPrompt("You are a planner. Given a topic, outline 3 key points. Be concise.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	writer, err := react.Builder().
		Name("Writer").
		SysPrompt("You are a writer. Expand the given points into a short paragraph.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	hub := msghub.New()
	hub.Register("planner", planner)
	hub.Register("writer", writer)

	results := hub.Broadcast(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("The future of AI agents.").
		Build())

	for name, resp := range results {
		fmt.Printf("=== %s ===\n%s\n\n", name, resp.GetTextContent())
	}
}
