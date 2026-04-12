# agentscope.go

[AgentScope Java](https://github.com/agentscope-ai/agentscope-java) 的 Go 语言实现 —— 一个生产级的 AI Agent 开发框架，助你使用 Go 构建基于大语言模型的智能应用。

## 概述

AgentScope Go 提供了构建智能 Agent 所需的一切，采用 ReAct（推理 + 行动）范式：工具调用、记忆管理、多 Agent 协作等功能一应俱全，并且全部使用地道的 Go 语言惯用法实现。

## 快速开始

**环境要求：** Go 1.22 或更高版本

```bash
go get github.com/linkerlin/agentscope.go
```

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    chatModel, _ := openai.Builder().
        APIKey(os.Getenv("OPENAI_API_KEY")).
        ModelName("gpt-4o-mini").
        Build()

    agent, _ := react.Builder().
        Name("Assistant").
        SysPrompt("You are a helpful AI assistant.").
        Model(chatModel).
        Build()

    response, _ := agent.Call(context.Background(), message.NewMsg().
        Role(message.RoleUser).
        TextContent("Hello! What can you help me with?").
        Build())

    fmt.Println(response.GetTextContent())
}
```

## 支持的模型

| 提供商 | 包路径 |
|--------|--------|
| OpenAI | `github.com/linkerlin/agentscope.go/model/openai` |
| DashScope (阿里云) | `github.com/linkerlin/agentscope.go/model/dashscope` |

任何兼容 OpenAI API 格式的服务都可以通过 `BaseURL` 配置使用。

## 核心包

| 包名 | 说明 |
|------|------|
| `message` | `Msg` 类型，支持多模态内容块（文本、图片、音频、视频、工具调用/结果、思考过程） |
| `model` | `ChatModel` 接口，支持流式响应 |
| `agent` | `Agent` 基础接口 |
| `agent/react` | ReAct Agent 实现 |
| `memory` | `Memory` 接口 + 内存实现 |
| `tool` | `Tool` 接口 + `FunctionTool` 适配器 |
| `session` | 会话管理 |
| `hook` | 钩子系统，支持人机协作 |
| `plan` | PlanNotebook，用于结构化多步骤任务管理 |

## 使用工具

```go
import "github.com/linkerlin/agentscope.go/tool"

myTool := tool.NewFunctionTool(
    "weather",
    "获取指定城市的当前天气",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "city": map[string]any{"type": "string"},
        },
        "required": []string{"city"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        city := input["city"].(string)
        return fmt.Sprintf("%s 天气晴朗，22°C", city), nil
    },
)

agent, _ := react.Builder().
    Name("WeatherBot").
    Model(chatModel).
    Tools(myTool).
    Build()
```

## 记忆管理

```go
import "github.com/linkerlin/agentscope.go/memory"

mem := memory.NewInMemoryMemory()
agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Memory(mem).
    Build()
```

## 钩子系统（人机协作）

```go
import "github.com/linkerlin/agentscope.go/hook"

loggingHook := hook.HookFunc(func(ctx context.Context, hCtx *hook.HookContext) (*hook.HookResult, error) {
    fmt.Printf("[%s] Agent: %s\n", hCtx.Point, hCtx.AgentName)
    return nil, nil
})

agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Hooks(loggingHook).
    Build()
```

## 计划笔记本

```go
import "github.com/linkerlin/agentscope.go/plan"

notebook := plan.NewPlanNotebook()
p := notebook.CreatePlan("研究任务")
notebook.AddStep(p.ID, "搜索信息")
notebook.AddStep(p.ID, "总结发现")

// 作为工具在 Agent 中使用
agent, _ := react.Builder().
    Name("Planner").
    Model(chatModel).
    Tools(notebook.AsTool()).
    Build()
```

## DashScope（阿里云）

```go
import "github.com/linkerlin/agentscope.go/model/dashscope"

chatModel, _ := dashscope.Builder().
    APIKey(os.Getenv("DASHSCOPE_API_KEY")).
    ModelName("qwen-max").
    Build()
```

## 示例

- [`examples/hello`](examples/hello/main.go) —— Agent 基础用法
- [`examples/tools`](examples/tools/main.go) —— 带计算工具的 Agent

## 许可证

Apache License 2.0 —— 详见 [LICENSE](LICENSE) 文件。
