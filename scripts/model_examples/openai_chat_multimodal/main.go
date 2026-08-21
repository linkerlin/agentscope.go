// openai_chat_multimodal demonstrates sending an image URL to a vision model.
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
	model := llmenv.MustChatModel()

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
