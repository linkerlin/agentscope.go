// dashscope_multimodal demonstrates sending an image URL to a DashScope vision model.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/dashscope"
)

func main() {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY is required")
	}

	model, err := dashscope.Builder().
		APIKey(apiKey).
		ModelName("qwen-vl-plus").
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
