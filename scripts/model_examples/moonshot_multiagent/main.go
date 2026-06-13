package main

import (
	"context"
	"fmt"
	"os"

	"github.com/linkerlin/agentscope.go/agent/react"
	"github.com/linkerlin/agentscope.go/message"
	"github.com/linkerlin/agentscope.go/model/moonshot"
	"github.com/linkerlin/agentscope.go/msghub"
)

func main() {
	apiKey := os.Getenv("MOONSHOT_API_KEY")
	if apiKey == "" {
		fmt.Println("请设置 MOONSHOT_API_KEY 环境变量")
		os.Exit(1)
	}

	ctx := context.Background()

	// 创建共享模型
	m, err := moonshot.Builder(apiKey).
		ModelName("moonshot-v1-8k").
		Build()
	if err != nil {
		fmt.Printf("创建模型失败: %v\n", err)
		os.Exit(1)
	}

	// 创建多个 ReAct Agent
	agent1, err := react.Builder().
		Name("Alice").
		Model(m).
		SysPrompt("你是一个乐于助人的助手，名叫 Alice。").
		Build()
	if err != nil {
		fmt.Printf("创建 Agent1 失败: %v\n", err)
		os.Exit(1)
	}

	agent2, err := react.Builder().
		Name("Bob").
		Model(m).
		SysPrompt("你是一个乐于助人的助手，名叫 Bob。").
		Build()
	if err != nil {
		fmt.Printf("创建 Agent2 失败: %v\n", err)
		os.Exit(1)
	}

	// 将 Agent 加入 Hub
	hub := msghub.New()
	hub.Register("Alice", agent1)
	hub.Register("Bob", agent2)

	// 创建初始消息
	msg := message.NewMsg().Role(message.RoleUser).TextContent("Hello, let's discuss the future of AI.").Build()

	// Alice 先回复
	resp1, err := agent1.Reply(ctx, msg)
	if err != nil {
		fmt.Printf("Alice 回复失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Alice: %s\n", resp1.GetTextContent())

	// Bob 基于 Alice 的回复继续对话
	resp2, err := agent2.Reply(ctx, resp1)
	if err != nil {
		fmt.Printf("Bob 回复失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Bob: %s\n", resp2.GetTextContent())
}
