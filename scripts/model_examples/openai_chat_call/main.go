// openai_chat_call demonstrates a simple Chat Completions call via OpenAI.
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
	chatModel := llmenv.MustChatModel()

	agent, err := react.Builder().
		Name("OpenAIAssistant").
		SysPrompt("You are a helpful assistant.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("Explain the benefits of Go for cloud services in one paragraph.").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
