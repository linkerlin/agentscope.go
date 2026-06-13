# 事件流（Event Stream）

AgentScope.Go 采用 V2 事件驱动架构，所有 Agent 操作都通过事件流进行通信。

## 核心概念

与 Python 版的 `AsyncGenerator[AgentEvent]` 不同，Go 版使用 **channel** 实现真事件流：

```go
ch := agent.ReplyStream(ctx, msg)
for evt := range ch {
    switch e := evt.(type) {
    case *event.TextBlockDeltaEvent:
        fmt.Print(e.Delta)  // 实时打印文本增量
    case *event.ToolCallStartEvent:
        fmt.Printf("Tool: %s\n", e.ToolName)
    case *event.ThinkingBlockDeltaEvent:
        fmt.Printf("[思考] %s", e.Delta)
    }
}
```

## 事件类型

| 事件 | 说明 |
|------|------|
| `ReplyStartEvent` / `ReplyEndEvent` | 回复开始/结束 |
| `TextBlockStartEvent` / `TextBlockDeltaEvent` / `TextBlockEndEvent` | 文本块 |
| `ThinkingBlockStartEvent` / `ThinkingBlockDeltaEvent` / `ThinkingBlockEndEvent` | 思考过程 |
| `ToolCallStartEvent` / `ToolCallDeltaEvent` / `ToolCallEndEvent` | 工具调用 |
| `ToolResultStartEvent` / `ToolResultTextDeltaEvent` / `ToolResultEndEvent` | 工具结果 |
| `ModelCallStartEvent` / `ModelCallEndEvent` | 模型调用 |
| `ExceedMaxItersEvent` | 超出最大迭代次数 |

## AG-UI 协议

事件流可通过 AG-UI 协议与前端 UI 对接：

```go
// SSE 端点自动支持 ?protocol=agui
GET /v2/chat/stream?protocol=agui
```

Python 版的 React Studio UI 可直接连接 Go Gateway。

## 与 Python 版对比

| | Python | Go |
|--|--------|-----|
| 流式接口 | `async for evt in agent.reply_stream(msg)` | `for evt := range agent.ReplyStream(ctx, msg)` |
| 背压控制 | 手动 | channel 天然背压 |
| 并发工具 | `asyncio.gather` | `errgroup` + goroutine |
| 序列化 | `model_dump_json()` | `event.MarshalEvent()` |
