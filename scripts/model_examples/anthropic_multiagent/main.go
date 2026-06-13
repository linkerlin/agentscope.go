package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/anthropic"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("请设置 ANTHROPIC_API_KEY 环境变量")
		os.Exit(1)
	}

	model, err := anthropic.NewBuilder().
		APIKey(apiKey).
		ModelName("claude-3-5-sonnet-20241022").
		Build()
	if err != nil {
		fmt.Printf("创建模型失败: %v\n", err)
		os.Exit(1)
	}

	agent1, err := react.Builder().
		Name("Alice").
		SysPrompt("你是一个乐于助人的助手，擅长回答问题。").
		Model(model).
		Build()
	if err != nil {
		fmt.Printf("创建 Agent1 失败: %v\n", err)
		os.Exit(1)
	}

	agent2, err := react.Builder().
		Name("Bob").
		SysPrompt("你是一个批判性思考者，喜欢提出不同观点。").
		Model(model).
		Build()
	if err != nil {
		fmt.Printf("创建 Agent2 失败: %v\n", err)
		os.Exit(1)
	}

	hub := msghub.New()
	hub.Register("Alice", agent1)
	hub.Register("Bob", agent2)

	msg := message.NewMsg().Role(message.RoleUser).TextContent("请讨论一下人工智能的未来发展趋势。").Build()
	responses := hub.Broadcast(context.Background(), msg)
	for name, resp := range responses {
		fmt.Printf("[%s] %s\n", name, resp.GetTextContent())
	}
}
