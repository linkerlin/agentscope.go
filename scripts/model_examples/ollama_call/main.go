// ollama_call demonstrates a simple call via a local Ollama server.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/ollama"
)

func main() {
	chatModel, err := ollama.NewBuilder().
		BaseURL("http://127.0.0.1:11434/v1").
		ModelName("llama3.2").
		Build()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := react.Builder().
		Name("LocalAssistant").
		SysPrompt("You are a helpful assistant running locally via Ollama.").
		Model(chatModel).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := agent.Call(context.Background(), message.NewMsg().
		Role(message.RoleUser).
		TextContent("What can you help me with?").
		Build())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.GetTextContent())
}
