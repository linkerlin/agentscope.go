// deepseek_call demonstrates a simple call via DeepSeek.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/deepseek"
)

func main() {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY is required")
	}

	chatModel, err := deepseek.Builder(apiKey).
		ModelName(deepseek.ModelChat).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("DeepSeekAssistant").
		SysPrompt("You are a helpful assistant powered by DeepSeek.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Write a haiku about programming.").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
