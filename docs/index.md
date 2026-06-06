# AgentScope.Go

> **高性能、强类型、云原生**的 Go 语言 Agent 框架，与 AgentScope Python v2 跨语言互操作。

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
| **Workspace 沙箱** | Local / Docker / E2B（待实现）三种执行环境隔离 |
| **权限引擎** | glob/regex/substring 规则匹配，支持 ALLOW/DENY/ASK/PASSTHROUGH |
| **多租户 Gateway** | HTTP + SSE + WebSocket，支持 API Key / JWT 认证 |
| **MCP 集成** | Client + Server 适配器，支持动态 schema 发现 |

---

## 下一步

- [核心概念](concepts.md) — 理解事件系统、AgentState、Workspace
- [部署指南](deployment.md) — 单机、Docker、K8s 部署
