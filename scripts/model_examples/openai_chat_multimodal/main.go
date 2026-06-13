// openai_chat_multimodal demonstrates sending an image URL to a vision model.
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
		log.Fatal("OPENAI_API_KEY is required")
	}

	model, err := openai.Builder().
		APIKey(apiKey).
		ModelName("gpt-4o-mini").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("VisionAssistant").
		SysPrompt("You are a helpful assistant that can analyze images.").
		Model(model).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	imageURL := "https://upload.wikimedia.org/wikipedia/commons/thumb/0/05/Go_Logo_Blue.svg/1200px-Go_Logo_Blue.svg.png"

	msg := message.NewMsg().
		Role(message.RoleUser).
		TextContent("What is in this image?").
		Content(message.NewImageBlock(imageURL, "", "image/png")).
		Build()

	resp, err := agent.Call(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
