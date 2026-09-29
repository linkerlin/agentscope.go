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

    card := a2a.AgentCard{
        Name:         "coder",
        Description:  "A coding assistant agent",
        URL:          "http://localhost:9000",
        Version:      "1.0.0",
        Capabilities: []string{"streaming"},
    }

    server := a2a.NewServer(card, a2a.NewV2AgentAdapter(agent), nil) // nil store → in-memory
    log.Fatal(http.ListenAndServe(":9000", server))
}
```

访问 `http://localhost:9000/.well-known/agent.json` 即可获取 AgentCard。

---

## 3. 发送任务

### 非流式

```go
client := a2a.NewHTTPClient("http://localhost:9000")
reply, err := client.Send(ctx, &a2a.Message{
    Role:    "user",
    Content: "Write a Go function that reverses a string.",
})
```

### 流式

```go
ch, err := client.SendSubscribe(ctx, &a2a.Message{
    Role:    "user",
    Content: "Write a Go function that reverses a string.",
})
for msg := range ch {
    fmt.Println("chunk:", msg.Content)
}
```

> 服务端任务的轮询等待与取消见 `a2a/http_client.go` 的 `WaitForTask` / `CancelTask`。

---

## 4. Registry 动态发现

```go
registry := a2a.NewRegistry()
registry.Register(card)
registry.StartBackgroundHealthCheck(ctx, 30*time.Second) // 30s 健康检查

// 枚举已注册条目并自行过滤
for _, entry := range registry.List() {
    if strings.Contains(entry.Card.Description, "coding") {
        // entry.Card.URL 可用于构造 a2a.NewHTTPClient(...)
    }
}
```

---

## 5. 与 Gateway 集成

A2A Server 是独立的 `http.Handler`（内置 `/.well-known/agent.json`、
`/task/send`、`/task/sendSubscribe`、`/task/cancel`、`/task/` 路由），
可与 gateway 进程共存、挂到同一个 mux：

```go
mux := http.NewServeMux()
mux.Handle("/a2a/", a2aServer) // gateway 的 srv 也可作为 handler 挂载
http.ListenAndServe(":9000", mux)
```

---

## 6. 安全建议

- A2A Server 应启用 HTTPS
- 对 `/task/send` 等端点进行认证（API Key / JWT）
- 限制 AgentCard 暴露的 capabilities，避免暴露敏感工具
- 使用 Registry 的健康检查剔除不可达 Agent

---

## 7. 认证中间件

A2A Server 支持多种认证方式，通过中间件链灵活装配：

### API Key 认证

```go
import "github.com/linkerlin/agentscope.go/a2a/middleware"

apiKeyAuth := middleware.APIKeyAuth("X-API-Key", func(key string) bool {
    return key == os.Getenv("A2A_API_KEY")
})
server := a2a.NewServer(card, adapter, a2a.WithMiddleware(apiKeyAuth))
```

### Bearer Token 认证

```go
bearerAuth := middleware.BearerAuth(func(token string) bool {
    return token == os.Getenv("A2A_BEARER_TOKEN")
})
server := a2a.NewServer(card, adapter, a2a.WithMiddleware(bearerAuth))
```

### JWT 认证

```go
jwtAuth := middleware.JWTAuth("your-secret-key", middleware.JWTConfig{
    Issuer:   "agentscope",
    Audience: "a2a",
})
server := a2a.NewServer(card, adapter, a2a.WithMiddleware(jwtAuth))
```

---

## 8. 限流器

内置令牌桶限流器，保护 A2A 端点免受突发流量冲击：

```go
rateLimiter := middleware.TokenBucketLimiter(middleware.TokenBucketConfig{
    Rate:  10, // 每秒 10 个请求
    Burst: 20, // 最多突发 20 个请求
})
server := a2a.NewServer(card, adapter, a2a.WithMiddleware(rateLimiter))
```

按客户端 IP 限流：

```go
ipLimiter := middleware.TokenBucketPerIP(middleware.TokenBucketConfig{
    Rate:  5,
    Burst: 10,
})
server := a2a.NewServer(card, adapter, a2a.WithMiddleware(ipLimiter))
```

---

## 9. WebSocket 实时推送

除 SSE 外，A2A Server 也支持 WebSocket 双向实时通信（`a2a/websocket.go`）：

```go
a2aServer := a2a.NewServer(card, a2a.NewAgentAdapter(agent), nil)
wsServer := a2a.NewWebSocketServer(a2aServer)

mux := http.NewServeMux()
mux.Handle("/a2a/", a2aServer)
mux.HandleFunc("/a2a/ws", wsServer.HandleWebSocket)

log.Fatal(http.ListenAndServe(":9000", mux))
```

客户端用任意 WebSocket 库连接 `/a2a/ws`：消息是 JSON 信封
（`WebSocketMessage`），服务端自动处理 task 转发与 ack——协议字段见
`a2a/websocket.go` 的 `handleMessage` / `handleTaskSend`。CORS 白名单经
`a2a.SetWebSocketAllowedOrigins` 配置。

---

## 10. 认证中间件

```go
auth := a2a.NewAuthMiddleware()
auth.AddAPIKey(os.Getenv("A2A_API_KEY"), "ops")
auth.SetJWTSecret(os.Getenv("A2A_JWT_SECRET"))
auth.AddPublicPath("/.well-known/agent.json")

// 包装任意 http.Handler（包括 a2aServer）
handler := auth.Middleware(a2aServer)
```

---

## 11. SecureServer 一键装配

`SecureServer` 将认证（API key / JWT）与限流打包为预设配置：

```go
auth := a2a.NewAuthMiddleware()
auth.AddAPIKey(os.Getenv("A2A_API_KEY"), "ops")

secure := a2a.NewSecureServer(card, a2a.NewAgentAdapter(agent), nil).
    WithAuth(auth)

log.Fatal(http.ListenAndServe(":9000", secure))
```

> 默认限流 100 rps / burst 200，可用 `WithRateLimit` 覆盖；认证规则见
> `a2a/middleware.go` 的 `AuthMiddleware` 一族（API key / JWT / 公开路径）。

---

## 12. 相关文件

- `a2a/server.go`
- `a2a/client.go`
- `a2a/registry.go`
- `a2a/v2_adapter.go`
- `a2a/middleware/auth.go`
- `a2a/middleware/ratelimit.go`
- `a2a/middleware/cors.go`
- `a2a/middleware/logger.go`
- `a2a/websocket.go`
- `a2a/secure_server.go`
- `examples/a2a/main.go`
