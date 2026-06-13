package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/ollama"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	// 1. 检查环境变量
	if os.Getenv("OLLAMA_BASE_URL") == "" {
		log.Fatal("OLLAMA_BASE_URL environment variable is required")
	}

	// 2. 使用 Builder 模式创建模型
	m, err := ollama.NewBuilder().
		BaseURL(os.Getenv("OLLAMA_BASE_URL")).
		ModelName("llama3.2").
		Build()
	if err != nil {
		log.Fatalf("failed to build model: %v", err)
	}

	// 3. 使用 react.Builder 创建 Agent
	alice, err := react.Builder().
		Name("Alice").
		SysPrompt("You are Alice, a helpful assistant.").
		Model(m).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	bob, err := react.Builder().
		Name("Bob").
		SysPrompt("You are Bob, a helpful assistant.").
		Model(m).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// 4. 使用 MsgHub 创建多 Agent 对话
	hub := msghub.New()
	hub.Register("Alice", alice)
	hub.Register("Bob", bob)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("Hello everyone! Let's discuss the weather.").Build()
	resp := hub.Broadcast(context.Background(), msg)
	for name, reply := range resp {
		fmt.Printf("[%s] %s\n", name, reply.GetTextContent())
	}
}
