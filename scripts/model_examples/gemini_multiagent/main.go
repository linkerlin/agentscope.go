// gemini_multiagent demonstrates a multi-agent conversation using MsgHub with Google Gemini.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/gemini"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}

	model, err := gemini.NewBuilder().
		APIKey(apiKey).
		ModelName("gemini-1.5-flash").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	planner, err := react.Builder().
		Name("Planner").
		SysPrompt("You are a planner. Given a topic, outline 3 key points. Be concise.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	writer, err := react.Builder().
		Name("Writer").
		SysPrompt("You are a writer. Expand the given points into a short paragraph.").
		Model(model).
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
