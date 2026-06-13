package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/dashscope"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY environment variable is not set")
	}

	ctx := context.Background()

	// 创建共享模型
	model, err := dashscope.Builder().
		APIKey(apiKey).
		ModelName("qwen-max").
		Build()
	if err != nil {
		log.Fatalf("failed to create model: %v", err)
	}

	// 创建多个 Agent
	agent1, err := react.Builder().
		Name("Alice").
		SysPrompt("You are Alice, a helpful assistant.").
		Model(model).
		Build()
	if err != nil {
		log.Fatalf("failed to create agent1: %v", err)
	}

	agent2, err := react.Builder().
		Name("Bob").
		SysPrompt("You are Bob, a creative assistant.").
		Model(model).
		Build()
	if err != nil {
		log.Fatalf("failed to create agent2: %v", err)
	}

	h := msghub.New()
	h.Register("Alice", agent1)
	h.Register("Bob", agent2)

	// 发送测试消息到 Hub
	testMsg := message.NewMsg().Role(message.RoleUser).TextContent("Hello everyone! What do you think about Go programming?").Build()
	responses := h.Broadcast(ctx, testMsg)
	for name, resp := range responses {
		fmt.Printf("Response from %s: %s\n", name, resp.GetTextContent())
	}
}
