# A2A 协议指南

AgentScope.Go 提供完整的 A2A（Agent-to-Agent）协议实现，支持分布式 Agent 注册、发现与通信。

## 核心组件

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Agent A   │◄───►│   Registry  │◄───►│   Agent B   │
│  (AgentCard)│     │  (Redis/内存)│     │  (AgentCard)│
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
                    ┌──────┴──────┐
                    │ ShardRouter │
                    │ (一致哈希)   │
                    └─────────────┘
```

## 快速开始

### 1. 启动 Registry

```go
import "github.com/linkerlin/agentscope.go/a2a"

// 内存注册中心
registry := a2a.NewRegistry(a2a.NewInMemoryRegistryStore())

// 或 Redis 分布式注册中心
redisStore, _ := a2a.NewRedisRegistryStore("redis://localhost:6379")
registry := a2a.NewRegistry(redisStore)
```

### 2. 注册 Agent

```go
agentCard := &a2a.AgentCard{
    Name:        "code-reviewer",
    Description: "Code review agent",
    URL:         "http://agent-a:8080",
    Capabilities: []string{"code_review", "go"},
}

registry.Register(ctx, agentCard)
```

### 3. 发现 Agent

```go
agents, _ := registry.Discover(ctx, "code_review")
```

### 4. 分片路由

```go
router := a2a.NewShardRouter(registry, 150) // 150 虚拟节点

// 自动刷新
router.AutoRefresh(ctx, 30*time.Second)

// 路由请求
target, _ := router.Route("task-123")
```

## 高级特性

### Watch 故障转移

```go
changes, _ := registry.Watch(ctx)
for change := range changes {
    switch change.Type {
    case a2a.RegisterChange:
        fmt.Printf("Agent registered: %s\n", change.AgentCard.Name)
    case a2a.RemoveChange:
        fmt.Printf("Agent removed: %s\n", change.AgentCard.Name)
    case a2a.HealthChange:
        fmt.Printf("Health changed: %s -> %v\n", change.AgentCard.Name, change.Healthy)
    }
}
```

### 任务管理

```go
task := &a2a.Task{
    ID:      uuid.New().String(),
    SessionID: sessionID,
    Message: message.NewMsg().TextContent("Review this code").Build(),
}

// 发送任务
stream, _ := a2a.SendTask(ctx, target.URL, task)
for evt := range stream {
    // 处理 SSE 事件流
}
```

## 与 Python 版对比

| 能力 | Python 2.0 | Go 2.0.0 |
|------|-----------|----------|
| A2A 实现 | ❌ Roadmap | ✅ 完整实现 |
| 分布式注册 | ❌ 无 | ✅ Redis + 分片 |
| 健康检查 | ❌ 无 | ✅ 自动 |
| 故障转移 | ❌ 无 | ✅ Watch + 自动刷新 |
