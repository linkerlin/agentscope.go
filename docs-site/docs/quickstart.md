# AgentScope.Go 快速上手

> 5 分钟内运行你的第一个 Agent

## 环境要求

- **Go 1.25** 或更高版本
- 一个 LLM API Key（OpenAI / Anthropic / Gemini 等）

## 安装

```bash
go get github.com/linkerlin/agentscope.go
```

## 第一个 Agent

创建 `main.go`：

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    // 1. 创建模型
    chatModel, _ := openai.Builder().
        APIKey(os.Getenv("OPENAI_API_KEY")).
        ModelName("gpt-4o-mini").
        Build()

    // 2. 创建 Agent
    agent, _ := react.Builder().
        Name("Assistant").
        SysPrompt("You are a helpful AI assistant.").
        Model(chatModel).
        Build()

    // 3. 调用
    response, _ := agent.Call(context.Background(), message.NewMsg().
        Role(message.RoleUser).
        TextContent("Hello! What can you help me with?").
        Build())

    fmt.Println(response.GetTextContent())
}
```

运行：

```bash
export OPENAI_API_KEY=sk-...
go run main.go
```

## 下一步

- [教程](tutorial.md) — 深入学习工具、事件流、多 Agent 编排
- [生产部署](deployment.md) — 将 Agent 部署为服务
