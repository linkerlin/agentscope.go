# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0-rc.1] - 2026-06-10

### Added — V2 事件驱动范式重构

- **事件系统 (`event/`)**：20+ 事件类型（`TextBlockDelta`/`ThinkingBlockDelta`/`ToolCallDelta` 等）+ `event.Bus` + `MetricsHandler` HTTP 端点，与 PyV2 JSON 字段完全对齐。
- **真事件流 (`agent/react`)**：`ReplyStream() -> <-chan event.AgentEvent`，Channel 驱动流式消费，支持 HITL 挂起/恢复、并发工具执行（errgroup）。
- **AgentState 状态机**：`reply_id`/`cur_iter`/`cur_summary` 可序列化到 Redis，`InjectEvent()` 恢复协议，跨请求 Agent 状态持续。
- **AG-UI Protocol (`gateway/agui.go`)**：完整的 AgentEvent → AG-UI 事件映射，`?protocol=agui` / `X-Protocol: agui` 查询参数切换，PyV2 Studio UI 即插即用。

### Added — 生产级能力

- **Workspace 沙箱 (`workspace/`)**：
  - `LocalWorkspace` + `DockerWorkspace`（含 `RenderDockerfile`/`BuildImage`/`HealthCheck`）
  - `E2BWorkspace`：完整 FS/命令操作（envd Connect-RPC），25 测试
  - MCP Gateway HTTP 代理 + `GatewayMCPClient` 主机侧适配
  - `Offloader` 协议（大上下文/工具结果外存卸载）
- **权限引擎 (`permission/`)**：规则引擎（glob/regex/substring）+ 4 种决策模式 + Bash AST 解析器（启发式 + 可选 tree-sitter）
- **中间件链**：
  - Agent 级 `MiddlewareBase`（`on_reply/on_reasoning/on_acting/on_model_call/on_system_prompt`）
  - Toolkit 级洋葱链（Logging + Metrics + Permission + Tracing + Offload）
- **Formatter 层扩展**：`ThinkingFormatter` + `extractThinkingBlocks` + MultiAgent 变体（9 后端全部覆盖）
- **Context Compression**：`memory/compactor.go` + `tool_result_compactor.go`，ReAct loop 自动集成
- **SubagentTool (`tool/subagent/`)**：递归 Agent 调用 + 嵌套深度限制（PyV2 无对等实现）
- **文件读缓存 (`tool/file/cache.go`)**：LRU 缓存 + mtime 感知失效

### Added — 服务化与生态

- **多租户 Service (`service/`)**：`Storage` 接口 + `MemoryStorage`/`RedisStorage` + JWT 认证 + AES-256-GCM 加密
- **Cron 调度器 (`schedule/`)**：基于 `robfig/cron/v3`，支持 `Schedule/Cancel/NextRun` + 重复 ID 替换
- **Gateway**：SSE `/v2/chat/stream` + WebSocket `/v2/chat/ws` + 多租户认证 + Session State 持久化闭环
- **A2A 协议**：V2Adapter + Registry 健康检查 + 动态发现（PyV2 roadmap 未实现）
- **可观测性 (`observability/`)**：OpenTelemetry + LangSmith 双链路追踪
- **内置 Agent 工具**：
  - `tool/task/` — TaskCreate/Get/List/Update（Agent 自管理任务）
  - `tool/schedule/` — ScheduleCreate/List/Stop/View + `StandardManager` 独立使用
  - `skill/skillviewer.go` — SkillViewer（Agent 运行时浏览 Skill）
  - `tool/web/` — WebFetch（HTTP GET 抓取 URL 内容，支持超时/截断/context 取消）
- **Tool Offload**：`gateway/tool_offload.go` + `offload_hints.go`，长耗时工具自动后台化 + ReAct hint 注入
- **异步任务池 (`async/pool.go`)**：固定 goroutine 池 + 任务状态跟踪 + 优雅关闭
- **Pipeline 并行执行 (`pipeline/parallel.go`)**：并发 Agent + 自定义聚合
- **E2E 集成测试 (`gateway/e2e_integration_test.go`)**：7 个测试覆盖多租户认证、SSE、Streamable HTTP、AG-UI、会话隔离
- **V2 事件流示例 (`examples/v2_event_stream/`)**：完整事件生命周期演示
- **Formatter 基准测试**：17 个 benchmark 覆盖 OpenAI/Anthropic/Gemini/DashScope + thinking 提取

### Added — 模型扩展

- **4 个新模型后端**：DeepSeek / Moonshot (Kimi) / xAI (Grok) / vLLM（vLLM 为 PyV2 无，Go 独有）
- **OpenAI Response API**：自定义 HTTP client + SSE 流式解析
- **ModelCard YAML**：35 个 YAML 声明式模型配置 + `/api/v1/models` HTTP API
- **AudioModel**：接口预留 + `OpenAITTS` 实现

### Changed

- Agent 输出从 `Call() -> Msg` 范式转为 `ReplyStream() -> <-chan AgentEvent`，保留 `Call/CallStream` 向后兼容
- 工具返回值统一为 `tool.Response` 规范类型
- `Msg.AppendEvent()` 驱动消息组装 + `SessionManager` 自动持久化
- 消息 JSON 序列化与 PyV2 完全兼容（`source` 嵌套 / `output` / `name` 字段）

### Fixed

- **OpenAI Streaming Panic**：nil check for `resp.Usage`
- **A2A Data Race**：`InMemoryTaskStore` 加 `sync.RWMutex`
- **Schedule 重复 ID**：`Schedule` 先 Remove 旧 entry 再添加
- Windows temp-directory cleanup in memory tests

## [0.1.0] - 2026-04-14

### Added

- **ReMe Memory System**: `ReMeFileMemory` + `ReMeVectorMemory` with file persistence, vector CRUD, hybrid retrieval
- **Orchestrator Layer**: `MemoryOrchestrator` coordinating summarizers and handlers
- **Wrapper Generator**: Interactive CLI for generating Go wrappers around Python tools
- **Formatter Abstraction**: `Formatter` interface decoupling message-to-API conversion
- **AgentBase**: `Base` struct with shared lifecycle, hooks, state management
- **ToolResponse**: Standardized `*tool.Response` replacing bare `any`
- **Pipeline/MsgHub/Workflow**: Multi-agent orchestration primitives
- **A2A Protocol Stack**: SSE streaming, HTTPClient, task management
- **Gateway**: HTTP/SSE/WebSocket endpoints
- **Reflection**: `SelfReflectingAgent` writer-critic loop
- **BM25/FTS5**: Full-text search with `modernc.org/sqlite`
- **Multi-Backend VectorStore**: Qdrant, Chroma, ES, PGVector
- **EmbeddingCache**: LRU cache for embedding models
