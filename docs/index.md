# AgentScope.Go

> **高性能、强类型、云原生**的 Go 语言 Agent 框架，与 AgentScope Python v2 跨语言互操作。

**最新追赶亮点**（接近 Python v2）：
- 高层 `gateway.AppConfig` + `NewApp` 自动装配（workspace、标准工具、schedule restore、embedding cache 等）。
- 独立 `embedding/` 包（多 provider + FileCache）。
- 纯 Go 轻量 `examples/studio`（HTMX + 实时 SSE + auto tools 结果展示 + 完整 CRUD）。
- 更多 provider parity（Gemini / DashScope 等）。

---

## 快速开始

### 安装

```bash
go get github.com/linkerlin/agentscope.go
```

### 最小可用示例

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/linkerlin/agentscope.go/agent/react"
    "github.com/linkerlin/agentscope.go/memory"
    "github.com/linkerlin/agentscope.go/message"
    "github.com/linkerlin/agentscope.go/model/openai"
)

func main() {
    // 1. 创建模型
    model, err := openai.Builder().
        APIKey("YOUR_OPENAI_API_KEY").
        ModelName("gpt-4o-mini").
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 2. 创建 Agent
    agent, err := react.Builder().
        Name("assistant").
        Model(model).
        Memory(memory.NewInMemoryMemory()).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 3. 对话
    ctx := context.Background()
    msg := message.NewMsg().Role(message.RoleUser).TextContent("你好，Go 世界的 Agent！").Build()
    resp, err := agent.Call(ctx, msg)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.GetTextContent())
}
```

### V2 事件流（推荐）

AgentScope.Go V2 的核心是**事件驱动范式**：

```go
ch, err := agent.ReplyStream(ctx, msg)
for ev := range ch {
    fmt.Printf("[%s] %s\n", ev.EventType(), ev.ReplyID())
}
```

支持 20+ 种细粒度事件：`TextBlockDelta`、`ToolCallStart`、`RequireUserConfirm` 等，与 Python v2 Studio UI 协议对齐。

---

## 核心特性

| 特性 | 说明 |
|------|------|
| **事件驱动** | 20+ 细粒度事件类型，真流式输出 |
| **挂起-恢复** | HITL（Human-in-the-Loop）支持，Agent 可在工具调用前等待用户确认 |
| **ReMe Memory** | 领先 Python 参考实现的长期记忆系统，支持 5 种向量后端 + Hybrid Search |
| **Workspace 沙箱** | Local / Docker / E2B 三种执行环境隔离 |
| **权限引擎** | glob/regex/substring 规则匹配，支持 ALLOW/DENY/ASK/PASSTHROUGH |
| **多租户 Gateway** | HTTP + SSE + WebSocket，支持 API Key / JWT 认证 + Session 持久化 |
| **MCP 集成** | Client + Server 适配器，支持动态 schema 发现 |
| **内置工具** | Read / Write / Edit / Insert / Glob / Grep / Shell / Subagent / WebFetch / JSON |
| **模型后端** | 10 个后端：OpenAI, Anthropic, Gemini, DashScope, Ollama, DeepSeek, vLLM, Moonshot, xAI, OpenAI Response |
| **多 Agent 编排** | Pipeline / Parallel / MsgHub / Workflow / Reflection / MapReduce |
| **异步任务池** | 固定 goroutine 池 + 任务状态跟踪 |
| **文档加载器** | TextLoader / DirLoader（RAG 前置） |
| **可观测性** | OpenTelemetry + LangSmith 双链路追踪 |
| **结构化输出** | json_object / json_schema 响应格式 |
| **跨语言互操作** | 与 Python v2 事件 JSON 格式对齐，契约测试验证 |

---

## 模型后端

```go
import (
    "github.com/linkerlin/agentscope.go/model/openai"
    "github.com/linkerlin/agentscope.go/model/anthropic"
    "github.com/linkerlin/agentscope.go/model/deepseek"
    "github.com/linkerlin/agentscope.go/model/moonshot"
    "github.com/linkerlin/agentscope.go/model/xai"
    "github.com/linkerlin/agentscope.go/model/vllm"
)

// 所有后端遵循相同的 Builder 模式
model, _ := openai.Builder().APIKey(key).ModelName("gpt-4o").Build()
model, _ := deepseek.Builder().APIKey(key).Build()
model, _ := moonshot.Builder().APIKey(key).Build()
model, _ := vllm.Builder().APIKey(key).BaseURL("http://localhost:8000/v1").Build()
```

---

## 下一步

- [教程](tutorial.md) — 从入门到生产的 5 步教程
- [核心概念](concepts.md) — 理解事件系统、AgentState、Workspace
- [API 参考](api-reference.md) — 完整接口速查
- [部署指南](deployment.md) — 单机、Docker、K8s 部署
