务必使用中文进行思考、推理和输出！
======

# AgentScope Go 开发备忘录

## 项目概述

本项目是 [AgentScope Java](https://github.com/agentscope-ai/agentscope-java) 的 Go 语言实现，采用地道的 Go 惯用法构建生产级 AI Agent 框架。

## 已完成的扩展功能

### 1. Plan 持久化 (`plan/storage.go`)
- `Storage` 接口：支持保存/加载/列出/删除 Plan
- `InMemoryStorage`：内存实现
- `JSONFileStorage`：基于 JSON 文件的持久化实现
- `EnhancedPlanNotebook`：自动保存的 Plan 管理器

### 2. WebSocket Transport (`model/transport`)
- `WebSocketTransport` 接口 + `GorillaWebSocketTransport` 实现
- 支持 `Chat` 和 `ChatStream` 的流式传输

### 3. 多模态工具 (`tool/multimodal`)
- `OpenAIMultiModalTool`：通过 `go-openai` SDK 集成 DALL-E + GPT-4o Vision
- `DashScopeMultiModalTool`：通过原生异步 HTTP 集成通义万相/通义千问 Vision
- `dashscope_async.go`：通用异步任务轮询客户端
- 全部使用 `message.ContentBlock`（`ImageBlock`/`AudioBlock`/`DataBlock`/`VideoBlock`）封装结果

### 4. 中断与关闭策略
- `interruption` 包：`Source`（User/Timeout/System/Hook）、`Context`
- `shutdown` 包：`PartialReasoningPolicy`（Save/Discard）、`GracefulShutdownConfig`
- `agent.Base`：原子中断标志、`Interrupt()`/`InterruptWithMsg()`/`InterruptWithSource()`/`CheckInterrupted()`/`CreateInterruptContext()`
- `ReActAgent`：模型调用前后检查中断、checkpoints、`handleInterrupt` 路由（USER→恢复消息；SYSTEM→保存/丢弃→`ErrAgentClosed`）

### 5. A2A 协议完善 (`a2a`)
- `AgentAdapter`：桥接 `agent.Agent` ↔ `a2a.AgentRunner`/`StreamingAgentRunner`
- `POST /task/cancel` 端点
- SSE 格式修复（`event: task\ndata: <json>\n\n`）
- `HTTPClient.WaitForTask()` 轮询助手 + `CancelTask()`
- 异步任务使用 `context.Background()` 避免请求上下文取消导致任务失败

### 6. 额外 Hook 点 (`hook`)
- 新增 `HookPreCall`：在 `ReActAgent.replyInternal` 中 `buildHistory` 之前触发
- 支持 `InjectMessages`、提前 `Interrupt`/`Override`

### 7. 并发工具执行 (`agent/react`)
- 工具循环替换为 `errgroup.Group`
- 所有工具并行执行，结果收集到预分配切片保持原始 `toolCalls` 顺序
- 每个 goroutine 中触发 `HookBeforeTool`/`PreActingEvent`/`PostActingEvent`/`HookAfterTool`
- `AddToolCallResult`/`calledTools` 在 `g.Wait()` 后串行更新

### 8. 分布式 Agent 协调 (`dist`)
- `Registry`：TTL 条目 + `Discover()`/`AutoDiscover()` 自动刷新
- `Coordinator`：`Random`/`RoundRobin`/`Broadcast` 策略
- `SendTo(name)` 单点派发、`Broadcast()` 群发

### 9. 示例项目 (`examples/`)
- `examples/a2a`：A2A 服务端/客户端完整示例（含 mock model）
- `examples/interrupt`：中断策略演示（含 slow model）
- `examples/multimodal`：多模态工具接入演示

## 编码规范

- 所有包必须通过 `go test ./... -race`
- 优先使用 `golang.org/x/sync/errgroup` 进行并发控制
- 中断检查优先使用原子操作，配合 `sync.RWMutex` 保护复杂状态
- 多模态结果使用 `message.ContentBlock` 子类型封装
- 与 Java AgentScope 对齐：`InterruptContext` 字段、`GracefulShutdownConfig` 语义、`HookPreCall` 对应 `PRE_CALL`
