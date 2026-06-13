// moonshot_call demonstrates a simple call via Moonshot (Kimi).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/moonshot"
)

func main() {
	apiKey := os.Getenv("MOONSHOT_API_KEY")
	if apiKey == "" {
		log.Fatal("MOONSHOT_API_KEY is required")
	}

	chatModel, err := moonshot.Builder(apiKey).
		ModelName(moonshot.Model8K).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("KimiAssistant").
		SysPrompt("You are a helpful assistant powered by Kimi.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("请用一句话总结 Go 语言的特点。").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
