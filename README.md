# agentscope.go

[AgentScope Java](https://github.com/agentscope-ai/agentscope-java) 的 Go 语言实现 —— 一个生产级的 AI Agent 开发框架，助你使用 Go 构建基于大语言模型的智能应用。

## 概述

AgentScope Go 提供了构建智能 Agent 所需的一切，采用 ReAct（推理 + 行动）范式：工具调用、记忆管理、多 Agent 协作等功能一应俱全，并且全部使用地道的 Go 语言惯用法实现。

## 快速开始

**环境要求：** Go 1.25 或更高版本

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

| 提供商 | 包路径 | 说明 |
|--------|--------|------|
| OpenAI | `github.com/linkerlin/agentscope.go/model/openai` | GPT-4o / GPT-4o-mini / o1 / o3 等 |
| Anthropic | `github.com/linkerlin/agentscope.go/model/anthropic` | Claude 3.5 Sonnet / Opus / Haiku 原生 HTTP + SSE |
| Gemini | `github.com/linkerlin/agentscope.go/model/gemini` | Gemini 1.5 Flash / Pro 原生 HTTP + SSE |
| DashScope (阿里云) | `github.com/linkerlin/agentscope.go/model/dashscope` | 通义千问系列（OpenAI 兼容） |
| Ollama | `github.com/linkerlin/agentscope.go/model/ollama` | 本地开源模型（OpenAI 兼容） |

任何兼容 OpenAI API 格式的服务都可以通过 `BaseURL` 配置使用。

## 核心包

| 包名 | 说明 |
|------|------|
| `message` | `Msg` 类型，支持多模态内容块（文本、图片、音频、视频、工具调用/结果、思考过程） |
| `model` | `ChatModel` 接口，支持流式响应 |
| `agent` | `Agent` 基础接口与 `Base` 统一生命周期（Hook、流式事件、Usage 统计） |
| `agent/react` | ReAct Agent 实现，内嵌 `agent.Base` |
| `memory` | `Memory` 接口 + 内存实现 + ReMe 长期记忆 |
| `tool` | `Tool` 接口 + `FunctionTool` 适配器 + `tool.Response` 规范多媒体结果 |
| `formatter` | 独立的模型请求/响应格式化抽象层（OpenAI / Anthropic / Gemini / DashScope / Ollama） |
| `pipeline` | 顺序多 Agent 编排（Pipeline） |
| `msghub` | 广播式多 Agent 消息调度（Hub） |
| `workflow` | 高级多 Agent 编排：并行（Parallel）、条件（Condition）、循环（Loop） |
| `a2a` | A2A 协议最小实现：AgentCard、任务发送、流式订阅 |
| `gateway` | HTTP + SSE Gateway，支持浏览器实时对话 |
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

### 基础 Memory

```go
import "github.com/linkerlin/agentscope.go/memory"

mem := memory.NewInMemoryMemory()
agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Memory(mem).
    Build()
```

### ReMe 长期记忆（文件 + 向量）

```go
import "github.com/linkerlin/agentscope.go/memory"
import "github.com/linkerlin/agentscope.go/memory/handler"

// 创建向量记忆
v, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embedModel)

// 注入编排器，实现自动提取与检索
orch := handler.NewMemoryOrchestrator(personalSum, proceduralSum, toolSum, memTool, profileTool, historyTool, dedup)
v.SetOrchestrator(orch)

// 端到端自动提取个人/任务记忆并写入向量库
res, _ := v.SummarizeMemory(ctx, msgs, "alice", "coding_task", "")

// 统一检索
nodes, _ := v.RetrieveMemoryUnified(ctx, "Go 最佳实践", "alice", "coding_task", "", memory.RetrieveOptions{TopK: 5})
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

## 多模型后端示例

### Anthropic

```go
import "github.com/linkerlin/agentscope.go/model/anthropic"

chatModel, _ := anthropic.NewBuilder().
    APIKey(os.Getenv("ANTHROPIC_API_KEY")).
    ModelName("claude-3-5-sonnet-20241022").
    Build()
```

### Gemini

```go
import "github.com/linkerlin/agentscope.go/model/gemini"

chatModel, _ := gemini.NewBuilder().
    APIKey(os.Getenv("GEMINI_API_KEY")).
    ModelName("gemini-1.5-flash").
    Build()
```

### DashScope（阿里云）

```go
import "github.com/linkerlin/agentscope.go/model/dashscope"

chatModel, _ := dashscope.Builder().
    APIKey(os.Getenv("DASHSCOPE_API_KEY")).
    ModelName("qwen-max").
    Build()
```

### Ollama

```go
import "github.com/linkerlin/agentscope.go/model/ollama"

chatModel, _ := ollama.NewBuilder().
    BaseURL("http://127.0.0.1:11434/v1").
    ModelName("llama3.2").
    Build()
```

## 多 Agent 编排

### 顺序执行（Pipeline）

```go
import "github.com/linkerlin/agentscope.go/pipeline"

pipe := pipeline.New("ResearchPipe", plannerAgent, writerAgent)
resp, _ := pipe.Call(ctx, message.NewMsg().Role(message.RoleUser).TextContent("Go 并发模式").Build())
```

### 广播调度（MsgHub）

```go
import "github.com/linkerlin/agentscope.go/msghub"

hub := msghub.New()
hub.Register("coder", coderAgent)
hub.Register("reviewer", reviewerAgent)
results := hub.Broadcast(ctx, msg) // map[string]*message.Msg
```

### 并行 / 条件 / 循环（Workflow）

```go
import "github.com/linkerlin/agentscope.go/workflow"

// 并行：让两个 Agent 同时处理，合并结果
par := workflow.NewParallel("DualCheck", nil, agentA, agentB)

// 条件：根据输入内容决定走哪个分支
cond := workflow.NewCondition("Router",
    func(m *message.Msg) bool { return strings.Contains(m.GetTextContent(), "urgent") },
    urgentAgent, normalAgent)

// 循环：反复优化直到满足质量条件
loop := workflow.NewLoop("Refiner", editorAgent,
    func(m *message.Msg) bool { return !strings.Contains(m.GetTextContent(), "FINAL") },
    5)
```

## 实时对话 Gateway

```go
import "github.com/linkerlin/agentscope.go/gateway"

srv := gateway.NewServer(agent)
http.ListenAndServe(":8080", srv)
```

- `POST /chat` —— 非流式对话，请求体 `{"text":"..."}`，返回 JSON。
- `POST /chat/stream` —— SSE 流式对话，浏览器可用 `EventSource` 接收增量回复。

## 示例

- [`examples/hello`](examples/hello/main.go) —— Agent 基础用法
- [`examples/tools`](examples/tools/main.go) —— 带计算工具的 Agent
- [`examples/anthropic`](examples/anthropic/main.go) —— 使用 Claude 后端的 Agent
- [`examples/gemini`](examples/gemini/main.go) —— 使用 Gemini 后端的 Agent
- [`examples/pipeline`](examples/pipeline/main.go) —— 多 Agent 顺序编排（Pipeline）
- [`examples/msghub`](examples/msghub/main.go) —— 广播式多 Agent 消息调度
- [`examples/workflow`](examples/workflow/main.go) —— 并行 + 条件 + 循环工作流
- [`examples/gateway`](examples/gateway/main.go) —— HTTP + SSE 实时对话 Gateway
- [`examples/reme/file`](examples/reme/file/main.go) —— ReMe 文件型记忆（ReMeLight）
- [`examples/reme/vector`](examples/reme/vector/main.go) —— ReMe 向量记忆检索
- [`examples/reme/orchestrator`](examples/reme/orchestrator/main.go) —— ReMe Orchestrator 端到端（提取 + 检索 + Profile）

## 许可证

Apache License 2.0 —— 详见 [LICENSE](LICENSE) 文件。
