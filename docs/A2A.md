# A2A 协议使用指南

AgentScope.Go 提供完整的 Agent-to-Agent (A2A) 协议实现，支持 Agent 发现、任务发送、SSE 流式和动态注册中心。

---

## 1. 核心概念

| 概念 | 说明 |
|------|------|
| **AgentCard** | Agent 的元信息卡片，暴露于 `/.well-known/agent.json` |
| **Task** | A2A 任务单元，包含输入消息、状态、产出物 |
| **SSE Stream** | 任务执行过程的实时事件流 |
| **Registry** | Agent 注册中心，支持动态发现与健康检查 |
| **V2Adapter** | 将 V2 Agent 桥接为 A2A Server |

---

## 2. 启动 A2A Server

```go
package main

import (
    "log"
    "net/http"

    "github.com/linkerlin/agentscope.go/a2a"
    "github.com/linkerlin/agentscope.go/agent/react"
)

func main() {
    agent, _ := react.Builder().
        Name("coder").
        Model(model).
        Build()

    card := &a2a.AgentCard{
        Name:        "coder",
        Description: "A coding assistant agent",
        URL:         "http://localhost:9000",
        Version:     "1.0.0",
        Capabilities: a2a.Capabilities{
            Streaming: true,
        },
    }

    server := a2a.NewServer(card, a2a.NewV2Adapter(agent))
    log.Fatal(http.ListenAndServe(":9000", server))
}
```

访问 `http://localhost:9000/.well-known/agent.json` 即可获取 AgentCard。

---

## 3. 发送任务

### 非流式

```go
client := a2a.NewClient("http://localhost:9000")
task, err := client.SendTask(ctx, &a2a.Task{
    ID: "task-1",
    Message: a2a.Message{
        Role: "user",
        Parts: []a2a.Part{
            {Type: "text", Text: "Write a Go function that reverses a string."},
        },
    },
})
```

### 流式

```go
ch, err := client.SendTaskSubscribe(ctx, task)
for event := range ch {
    switch e := event.(type) {
    case *a2a.TaskStatusUpdateEvent:
        fmt.Println("Status:", e.Status.State)
    case *a2a.TaskArtifactUpdateEvent:
        fmt.Println("Artifact:", e.Artifact.Parts[0].Text)
    }
}
```

---

## 4. Registry 动态发现

```go
registry := a2a.NewRegistry(30 * time.Second) // 30s 健康检查间隔
registry.Register(card)

// 发现所有健康 Agent
agents := registry.ListHealthy()

// 按能力过滤
coders := registry.Filter(func(c *a2a.AgentCard) bool {
    return strings.Contains(c.Description, "coding")
})
```

---

## 5. 与 Gateway 集成

A2A Server 可以独立部署，也可以嵌入 Gateway：

```go
srv := gateway.NewApp(gateway.AppConfig{
    Agent: agent,
})
srv.RegisterA2ARoutes(card) //  hypothetical; check actual API
```

> 实际 API 请参考 `gateway/server.go` 和 `a2a/server.go` 中的路由注册方法。

---

## 6. 安全建议

- A2A Server 应启用 HTTPS
- 对 `/task/send` 等端点进行认证（API Key / JWT）
- 限制 AgentCard 暴露的 capabilities，避免暴露敏感工具
- 使用 Registry 的健康检查剔除不可达 Agent

---

## 7. 相关文件

- `a2a/server.go`
- `a2a/client.go`
- `a2a/registry.go`
- `a2a/v2_adapter.go`
- `examples/a2a/main.go`
