# Migration Guide

本指南说明如何在不同版本或从 Python AgentScope 迁移到 AgentScope.Go。

---

## 从 Python AgentScope 2.0 迁移到 AgentScope.Go

### 架构差异

| Python | Go | 说明 |
|--------|-----|------|
| `agentscope.agent.Agent` | `agent/react.ReActAgent` / `agent.Agent` | Go 同时提供 V1 (`Call`) 与 V2 (`ReplyStream`) 接口 |
| `agentscope.app.create_app` | `gateway.NewApp` | Go 版通过 `AppConfig` 一键装配 |
| `agentscope.message.Msg` | `message.Msg` + `ContentBlock` | JSON 格式已对齐 |
| `agentscope.event.AgentEvent` | `event.AgentEvent` | 字段已对齐，channel 替代 AsyncGenerator |
| `agentscope.tool.Toolkit` | `toolkit.Toolkit` | 接口相似 |
| `agentscope.state.AgentState` | `state.AgentState` | 可序列化到 Redis/JSONFile |
| `agentscope.permission.PermissionEngine` | `permission.Engine` | 规则引擎对齐 |
| `agentscope.workspace.Workspace` | `workspace.Workspace` | Local/Docker/E2B 对齐 |

### 消息与事件

Python 使用 `reply_stream()` 返回 `AsyncGenerator[AgentEvent]`；Go 使用：

```go
ch := agent.ReplyStream(ctx, msg)
for evt := range ch {
    switch evt.Type {
    case event.EventTypeTextBlockDelta:
        // 处理文本增量
    case event.EventTypeToolCallStart:
        // 处理工具调用
    }
}
```

### 服务启动

Python：

```python
from agentscope.app import create_app
app = create_app(storage=RedisStorage(...))
```

Go：

```go
srv := gateway.NewApp(gateway.AppConfig{
    Storage:          service.NewRedisStorage(...),
    WorkspaceBaseDir: "./workspaces",
    AutoStandardTools: true,
})
srv.RegisterAppRoutes(jwtAuth)
srv.Start()
defer srv.Close()
```

### 模型后端

Python 使用 `OpenAIChatModel(...)`；Go 使用 builder 模式：

```go
model, _ := openai.Builder().APIKey(key).ModelName("gpt-4o").Build()
```

### 记忆

Python 2.0 当前仅支持短期上下文压缩。Go 提供 ReMe 长期记忆：

```go
mem := memory.NewInMemoryMemory()
// 或
v, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embedModel)
```

---

## 从 AgentScope.Go v1.x 迁移到 v2.0.0

### Agent 接口变更

- `Call(ctx, msg) (*message.Msg, error)` 保留
- 新增 `ReplyStream(ctx, msg) <-chan event.AgentEvent`
- 建议新代码使用 V2 事件流；旧代码可继续用 `Call`

### 工具返回值

工具 handler 返回值建议统一为 `*tool.Response`：

```go
func myTool(ctx context.Context, input map[string]any) (*tool.Response, error) {
    return tool.NewResponse("result"), nil
}
```

### 模型构建器

v1 中部分模型使用直接构造。v2 统一为 builder 模式：

```go
// v1
model := &openai.Model{APIKey: key, ModelName: "gpt-4o"}

// v2
model, _ := openai.Builder().APIKey(key).ModelName("gpt-4o").Build()
```

### Gateway

v1 `gateway.NewServer(agent)` 仍可用，但推荐使用 `gateway.NewApp(AppConfig{})` 获得完整多租户、会话、调度、工作区能力。

---

## 从 v2.0.0-rc.x 迁移到 v2.0.0

- `embedding/` 成为独立顶级包，`memory/embedding` 已标记为 deprecated adapter
- `gateway.AppConfig` 新增 `AutoStandardTools`、`AutoToolOffload`、`EmbeddingCacheDir` 等字段
- `evolver/` 包为新增可选能力，不影响现有代码
- 事件类型 JSON 序列化保持稳定

---

## 获取更多帮助

- 查看 [README.md](./README.md)
- 查看 [docs/tutorial.md](./docs/tutorial.md)
- 提交 Issue 或 Discussion
