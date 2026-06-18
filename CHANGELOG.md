# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 2026-06-18

### Added — 对照 Python 参考实现的追赶 (演进方案 v3, Tier 1)
- **`tts/` 独立 TTS 包** (对齐 Python #1832)：`tts.Model` / `tts.RealtimeModel` 抽象（非实时 `Synthesize` + 实时 `Connect/Push/Close`）、`Options`/`Response`、DashScope 后端（CosyVoice/qwen3-tts 同步生成 API + 可注入 HTTP client）、OpenAI 适配器（复用既有 `model.OpenAITTS`，零重复）、`tts.ModelCard` + 内嵌 YAML 卡片（cosyvoice-v1 / qwen3-tts-flash / qwen3-tts-flash-realtime / tts-1 / tts-1-hd）、`ListModelCards`/`FindModelCards`。
- **`middleware/tts.go` TTSMiddleware** (对齐 Python #1832)：拦截 assistant 回复文本 → 合成语音 → 以 base64 `AudioBlock` 附加到回复消息；支持 `Strict` 错误传播模式，默认非致命（错误记入 `Metadata["tts_error"]`）。
- **`middleware/budget.go` BudgetControlMiddleware** (对齐 Python #1738)：按 reply 计的加权 token 预算（`InputTokenWeight*in + OutputTokenWeight*out`），超限注入 `HintBlock` + 强制 `tool_choice=none`；预算累加器经共享 `context.Context` 在 on_reply/on_model_call/on_reasoning 三链间传递（实例无状态、可跨 agent 共享）；token 用量优先 `msg.Usage`，回退 `msg.Metadata["usage"]`（流式）。
- **`middleware/longterm_memory.go` 长期记忆中间件** (对齐 Python #1775)：`LongTermMemory` 接口（`Search`/`Add`）+ 三模式（`static_control` 自动检索注入+回写 / `agent_control` 暴露工具 / `both`）；`InMemoryLongTermMemory`（线程安全，demo/测试）、`NewFuncLongTermMemory`（函数适配器，零耦合桥接 ReMe/外部 mem0）；`search_memory`/`add_memory` 工具；经框架修复在 on_reasoning 注入记忆 HintBlock。
- **`embedding/` ModelCard 化** (对齐 Python #1852)：`embedding.ModelCard` + 内嵌 per-provider YAML 卡片（OpenAI text-embedding-3-large/small、Gemini embedding-001、DashScope text-embedding-v3/v4 + 多模态、Ollama nomic-embed-text）+ `ListModelCards`/`FindModelCard`/`ModelCardsByProvider`，支撑 Studio 动态表单。
- **自定义 Agent 类注册表** (对齐 Python #1838)：`AgentFactory.RegisterAgentClass(name, builder)` + `AgentClassBuilder` 类型；`service.AgentConfig` 新增 `AgentClass`（默认 `"react"`）与 `SubagentTemplates` 字段；`createAgentRequest` 透传 `agent_class`。未注册类构建时报清晰错误。
- **gateway workspace→权限接线** (对齐 Python #1823)：`gateway/agent_session.go` 将会话工作区根路径注入 `permission.Context.WorkingDirs`，使 ACCEPT_EDITS 模式可在会话工作区内自动放行（修复"管道存在但未接线"缺陷）。

### Changed
- **中间件框架增强** (`agent/react/stream.go`)：`runModel` / `invokeModelChat` 的 final 闭包改为从 `input` 读取 `Messages`/`ChatOpts`，使 on_reasoning / on_model_call 中间件对请求的变更（如注入 hint、强制 tool_choice）真正生效——对齐 Python v2 MiddlewareBase 语义，且不破坏既有中间件（LoggingMiddleware 等不变）。
- **`gateway/agent_factory.go`**：提取 `buildAgentForClass` + `defaultReactAgentBuilder`，`Build`/`BuildFromTyped` 改为经 Agent 类注册表分发（默认行为不变，向后兼容）。

### Fixed
- **`plan/dag_executor.go` 数据竞争**：修复并行批次中多个 goroutine 无同步并发写 `plan.UpdatedAt`（`time.Time` 多字结构）与 `failedStep`/`failedErr` 的竞态；新增 scoped `sync.Mutex` 保护共享状态。
- **`async/pool.go` 任务 ID 碰撞**：`Submit` 改用原子递增计数器生成唯一 ID（`task_<seq>_<nano>`），消除高并发下 `time.Now().UnixNano()` 碰撞导致的 `TestPool_ManyTasks` 偶发失败。全量 `go test ./... -race` 恢复稳定全绿。
- **`memory/async_queue_test.go` 优先级测试竞态**：`TestAsyncTaskQueuePriority` 用 ready-gate 确保三任务全部入队后 worker 才开始处理，消除"worker 抢先处理首个低优先级任务"导致的偶发失败。

### Added — 对照 Python 参考实现的追赶 (演进方案 v3, Tier 3/4)
- **Agent Team 运行时 spawn + 权限继承**（对齐 #1833/#1815）：`gateway/agent_team.go` 新增 `AgentFactory.BuildSubagentTools`，将 leader 的 `SubagentTemplates` 在 `BuildSessionAgent` 中 spawn 为 `SubagentTool`；每个子 agent **继承 leader 的权限引擎**（#1815）并共享会话工作区，获得基础 file/shell 工具集；新增 `NewSessionWorkspace` 构造器。
- **`messagebus/` 分布式消息总线**（对齐 #1849）：`Bus` 接口 + `Message`；`LocalBus`（进程内、发布非阻塞、慢订阅者不拖累）+ `RedisBus`（Redis pub/sub，多进程/分布式）；`AppConfig.MessageBus` + `Server.WithMessageBus`/`MessageBus()` 接入，支撑跨进程 cancel/wake-up/offload 协调。
- **可运行示例**：`examples/longterm_memory`（三模式 + 回写）、`examples/tts`（TTSMiddleware + 卡片）、`examples/messagebus`（LocalBus + 内嵌 miniredis RedisBus）、`examples/agent_team`（子 agent 模板 spawn + 委托）。

## [2.0.0] - 2026-06-12

### Added — 社区治理与工程化
- 新增 `CONTRIBUTING.md`、`SECURITY.md`、`CODE_OF_CONDUCT.md`。
- 新增 GitHub Issue 模板（bug report / feature request / RFC）与 PR 模板。
- 新增 `MIGRATION.md` 与 `RELEASE_CHECKLIST.md`。
- 扩展 `docs/DEPLOYMENT.md`，补充 Docker、systemd、K8s、Redis 部署指南。
- 新增跨平台 CI（Windows / macOS）、覆盖率上报、`govulncheck` 安全扫描、stale 管理、PR title 校验工作流。
- 新增模型示例脚本库 `scripts/model_examples/`，覆盖主流模型后端。
- 补齐 `tool/schedule/` 单元测试。

### Changed / P3 工程收尾与架构试点
- **事件总线 race 根治**：修复 TestBus_Stress* DATA RACE（close(done) + select on done for consumers，Subscribe 扩展返回 done chan，保持 ch 不 close 避免 send-on-closed）。全 -race gate 采样通过（event stress 全绿 + 更多包）。
- **lint 配置与最后项清理**：.golangci.yml 稳定 v1 兼容，增加 memory/ exclude（errcheck/gosec/unused/ineffassign/gosimple/staticcheck），react/stream/formatter unparam，goimports source 修复（显式 -local）。gofmt 全局 0。
- **内存轻拆分试点启动**（原审阅报告架构建议）：创建 `memory/vector/` 子包，移动 LocalVectorStore、ChromaVectorStore、QdrantVectorStore impl（package vector，类型限定 memory.* 共享），父包 facade alias 保持公开 API 完全向后兼容（New*VectorStore、类型、Err*）。其他 vector store 保留 parent 待续。go build ./memory 通过，测试采样绿。后续可渐进移余下 + reme 等。
- **发布准备**：CHANGELOG 更新本次 P3（race、lint、split pilot），版本后续升级为 2.0.0 正式发布。
- **godoc 补齐**：补充 memory/registry.go、vector_store facade、gateway/app.go 等导出项注释。
- **Studio/e2e**：examples/studio 模板/代码小增强（添加版本显示注释），e2e 集成测试文档化。

### Fixed
- event/bus.go Unsubscribe/Subscribe 设计与测试一致性（done 信号优雅退出）。
- 多个 goimports 漂移（react、memory、gateway）。
- 配置 deprecation 与 v1/v2 兼容问题。

### Added — Phase 5: 追踪、文档、测试、发布
- **Tracing middleware 对齐**：增强 `TracingMiddlewareAdapter` (支持更多 OnCall/OnResult 集成)，添加使用示例。参考 Python `middleware/_tracing/`。
- 在 `examples/full_service` 中集成 `observability.NewTracedAgent` 演示 tracing + auto-assembly 组合。
- 补充测试：tracing adapter、embedding providers/cache、observability。
- 文档全面更新：README（新增 tracing middleware 章节）、CHANGELOG、AGENTS.md（架构+模块表）、DEV_PLAN_CATCHUP.md、tutorial.md（新增 tracing 小节）、deployment.md（生产 tracing 示例）、concepts.md、index.md。突出 Phase 5 tracing、auto bootstrap、embedding、Studio。
- 保持全量 `-race` 测试通过，构建验证。

### Added — 高层自动装配与生产级 bootstrap (Phase 1-2)
- `gateway/app.go` + `AppConfig`：`NewApp(cfg)` 一键装配 Storage、SessionManager、BackgroundTaskManager (含 schedule 自动 restore on Start)、WorkspaceManager (by WorkspaceBaseDir)、ToolOffload、默认 StandardTools 注入。
- 自动 `StandardTools` + `AutoStandardTools` / `AutoToolOffload` / `EmbeddingCacheDir`：为 static agent 和动态 per-session agents 自动提供 file/task/web/json + permission + cache。
- `examples/full_service` 和简化版 `production`：极简配置演示 auto-assembly + schedule 持久化 + 恢复。
- `gateway/standard_tools.go` + `WithFileCache` 等 helper。

### Added — Embedding 包 (Phase 3)
- 新顶级 `embedding/` 包：`NewOpenAI`、`NewOllama`、`NewGemini`、`NewDashScope`（支持多模态提示）、`WithFileCache`（gob + sha256，类似 Python FileEmbeddingCache）。
- 完全兼容 `model.EmbeddingModel`，可直接用于 `gateway.WithEmbeddingModel`。
- `memory/embedding` 已包装/迁移到新包，减少重复。
- Gateway 自动 cache 支持：`AppConfig.EmbeddingCacheDir` 自动 `embedding.WithFileCache`。
- 更新 `full_service` / `studio` 示例启用带 cache 的 embedding。

### Added — Studio 轻量 UI 打磨 (Phase 4)
- `examples/studio`：纯 Go + HTMX 实现。
  - 支持 Auth、Agents CRUD、Credentials (schemas 驱动)、Schedules、Chat。
  - 真实 SSE streaming (`/v2/chat/stream`) + 事件解析，实时展示 text deltas 和 **auto tools 实际调用结果**（`[AUTO TOOL] ...` / `[AUTO TOOL RESULT]`）。
  - Demo register + X-Demo-User 头支持快速测试。
  - 漂亮表格渲染 + delete + "Use in Chat" 联动。
- `studio/main.go` 默认启用全 auto-assembly (AutoStandardTools + Workspace + ToolOffload + EmbeddingCache)。
- 直接演示 Python Studio 风格的 auto tools + schedule restore 效果。

### Added — 其他追赶与增强
- Docker 模板增强：支持 pypi/src/node/full 多个 flavor + 专用模板 (RenderDockerfile)。
- 更多 credential provider 支持 + schemas 端点。
- 初始 static agent 默认使用 auto tools（与动态 session 一致）。
- 相关测试、构建、e2e 验证通过。

### Changed
- `AppConfig` / `NewApp` 成为推荐高层入口，显著降低“生产级 Agent Service”搭建门槛，接近 Python `create_app` + lifespan 体验。
- Embedding 成为一等公民包。

### Added — Phase 6: 对齐 ./evolver/ GEP 自演化优势 (进入新阶段)
- 新 `evolver/` 包（types + client + flow）：
  - `types.go`：Gene/Capsule/Task 等完整 Go 结构体 + Create/Validate（严格对齐 evolver/src/gep/schemas/gene.js, capsule.js, task.js + 实时 MCP 基因样本 + seed）。
  - 支持 category (repair/optimize/innovate/explore)、constraints、blast_radius、routing_hint、tool_policy、epigenetic、skill2gep source 等。
  - `client.go`：Evolver 接口（list/upsert/run/reflect/solidify/remember/recall/meeting/claim/complete/stats/safety）。MockEvolver（预加载真实基因）、RecordingEvolver（调用记录，对齐 Phase5 tracing）。
  - `gep.go`：NewGEPFlow + RunAndSolidify（完整 run→reflect→solidify 闭环）、DistillSkillToGene（skill2gep 风格）。
- `skill/skill.go`：AgentSkill.DistillToGene 一行委托（将 ad-hoc skill 蒸馏为可演化 Gene）。
- `memory/reme_types.go`：新增 MemoryTypeGene/Capsule/EvoEvent（支持 narrativeMemory / gene 记忆图风格）。
- `gateway/app.go`：AppConfig.EvolverEnabled 提示（通过现有 MCP 网关即可让 agent 调用 evolver 全部工具）。
- `examples/evolver/main.go`：可独立运行完整 demo，展示 GEP 闭环、distill、recall、meeting、recording calls、与 ReMe 结合。
- 验证：`go build ./...`、`go test ./evolver -race`、`go run ./examples/evolver` 全绿；输出清晰打印 signals/gene/capsule/distilled/recall 等。
- 文档/计划同步：DEV_PLAN_CATCHUP 新 Phase 6 详尽章节（含优势列表、互补策略、API 示例）；本 CHANGELOG；后续将更新 README/AGENTS/docs。

**定位**：agentscope.go 现在不仅追上 Python v2 生产体验，还通过 evolver GEP 对齐获得了工业级“策略基因驱动的自演化”能力，同时保留轻量 Go 实现与强大 ReMe/a2a 优势。

### Added — Phase 3: RAG 增强与 Rerank 集成
- 新 `rerank/` 包，提供统一的 `Reranker` 接口：
  - `NoopReranker`：透传候选，便于开关重排。
  - `CohereReranker`：调用 Cohere `/v2/rerank` API。
  - `JinaReranker`：调用 Jina AI `/v1/rerank` API。
  - `LocalReranker`：基于 `vector.EmbeddingModel` 的余弦相似度本地重排，无需外部服务。
- `ReMeVectorMemory.WithReranker(r)` 链式方法，将 Rerank 接入检索流程：先向量召回候选集，再 Rerank 精排并截断 TopK。
- 新增 Cookbook `cookbook/rag_with_rerank/`：演示两阶段 RAG + Cohere/Jina/Local 自动回退的 Rerank 后端。
- `rerank/` 单元测试：覆盖 Noop、Local、Cohere（mock HTTP server）、Jina（mock HTTP server）。

### Added — Phase 3: 向量数据库补齐
- 完整 REST API 向量存储连接器：`memory/vector/vector_store_milvus.go`、`vector_store_qdrant.go`、懒加载版 `vector_store_chroma.go`。
- `memory/vector/docker-compose.yml`（Milvus + Qdrant + Chroma）与 `integration_test.go`（`//go:build integration`）。
- `embedding.NewDashScopeMultimodal`：DashScope 多模态嵌入支持。

### Added — Phase 4: A2A 分布式注册中心与分片路由（初版）
- `a2a/store.go`：引入 `RegistryStore` 接口，默认提供线程安全的内存实现；`Registry` 重构为可插拔存储后端。
- `a2a/store_redis.go`：`RedisRegistryStore`（基于 `go-redis`），支持注册/查询/列表/删除/健康状态更新，使用 Redis Set 作为 URL 索引。
- `a2a/router.go`：`ShardRouter` 基于一致哈希实现请求分片，仅将流量路由到健康节点；支持虚拟副本以改善分布性。
- `a2a/watch.go`：`Registry.Watch(ctx)` 本地变化事件流；`RegistryChange`（register/remove/health）；`ShardRouter.AutoRefresh(ctx, interval)` 通过 Watch 即时刷新 + 轮询兜底，自动感知节点故障与恢复。
- `a2a/store_redis_test.go` / `a2a/router_test.go` / `a2a/watch_test.go`：覆盖 Redis CRUD、健康更新、路由确定性、跳过不健康节点、Watch 事件、自动刷新等场景。
- `examples/a2a_redis_registry/`：可独立运行示例，支持真实 Redis（`REDIS_URL`）或嵌入式 `miniredis`，演示注册、发现、分片路由、Watch 事件与自动故障切换。

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
