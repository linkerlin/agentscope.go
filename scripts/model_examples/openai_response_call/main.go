// openai_response_call demonstrates the OpenAI Responses API (o3 / o4-mini).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/openai_response"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	chatModel, err := openai_response.Builder().
		APIKey(apiKey).
		ModelName("o3").
		ThinkingEnable(true).
		ReasoningEffort("medium").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("ReasoningAssistant").
		SysPrompt("You are a careful reasoning assistant.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Solve: if a train travels 60 km in 30 minutes, what is its average speed in km/h? Show your reasoning.").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
