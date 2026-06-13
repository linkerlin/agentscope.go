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
		fmt.Println("Error: GEMINI_API_KEY environment variable not set")
		os.Exit(1)
	}

	model, err := gemini.NewBuilder().
		APIKey(apiKey).
		ModelName("gemini-1.5-flash").
		Build()
	if err != nil {
		fmt.Printf("Error building model: %v\n", err)
		os.Exit(1)
	}

	agent, err := react.Builder().
		Name("GeminiMultimodalAgent").
		Model(model).
		SysPrompt("You are a helpful assistant that can analyze images.").
		Build()
	if err != nil {
		fmt.Printf("Error building agent: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	msg := message.NewMsg().Role(message.RoleUser).TextContent("Describe this image in detail.").Build()
	msg.Content = append(msg.Content, message.NewImageBlock("https://upload.wikimedia.org/wikipedia/commons/thumb/4/47/PNG_transparency_demonstration_1.png/300px-PNG_transparency_demonstration_1.png", "", ""))

	resp, err := agent.Call(ctx, msg)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Response: %s\n", resp.GetTextContent())
}
