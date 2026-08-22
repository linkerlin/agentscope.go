务必使用中文进行思考、推理和输出！
======

# AgentScope Go 开发备忘录

## 项目概述

本项目是 [AgentScope](https://github.com/agentscope-ai/agentscope) 的 Go 语言实现，采用地道的 Go 惯用法构建生产级 AI Agent 框架。当前版本 **v2.5.1**（Phase 14 演进中：Console TUI / Workspace 服务化 / 治理-演化闭环已落地，目标 v2.6.0）。

## V2 架构总览

```
网关层     gateway/       HTTP/SSE/WebSocket/流式 HTTP + AG-UI Protocol + 多租户认证 + Tool Offload
          a2a/           Agent-to-Agent 协议 (Go 领先 PyV2)
服务层               service/       Storage 抽象 + CRUD + AES-GCM 加密 + Cipher + **Access Policy (跨用户资源共享)**
          schedule/      Cron 调度器 + 定时任务引擎
          runcontext/    运行时上下文注入 (Session/Tools/Agent)
状态层     state/         AgentState 可序列化存储 (JSONFile + Redis + 加密)
          session/       会话管理 (SessionManager + 事件缓冲 + 重放)
          interruption/  中断处理
          shutdown/      优雅关闭
消息层     message/       Msg 类型 + 多模态 ContentBlock (Text/Image/Audio/File 等)
编排层     pipeline/      流水线编排 (顺序 + 并行)
          workflow/      MapReduce/Condition/Loop/Parallel
          msghub/        消息中心广播
          reflection/    反思机制 (Writer+Critic)
Agent 层   agent/         V1/V2 接口 + Base 基类 + ReActAgent (事件流 + 状态机 + 结构化输出)
          middleware/     Agent 生命周期中间件链 (洋葱模型: Reply/Reasoning/**CheckPermission**/Acting/ModelCall/**CompressContext**/SystemPrompt 7钩子) + RAG/AgenticMemory/LongTermMemory/TTS/Budget/**Injection**
          subagent/      元工具: Agent 作为 Tool 递归委托
事件系统   event/         20+ 事件类型 + Bus + MetricsHandler
          hook/          经典 Hook (12 HookPoint) + StreamHook (9 事件) + JSONL Trace Exporter
能力层     model/         10+ 后端 + OpenAI Responses API + ModelCard YAML (35 卡片) + TTS/Audio + Router
          tool/          内置工具集 (file/shell/web/json/multimodal + Task/Schedule/Subagent)
          toolkit/       工具注册/执行 + MCP 适配 + 中间件链
          formatter/     3 独立 Formatter (OpenAI/Anthropic/Gemini) + 2 别名 + 11 MultiAgent 变体
          workspace/     Local/Docker/E2B + MCP Gateway + Offloader
          permission/    规则引擎 + Bash 复合命令拆分 (启发式/tree-sitter) + Shell 命令验证
          embedding/     独立 Embedding 包 (OpenAI/Ollama/Gemini/DashScope/DashScope多模态 + FileCache)
          tts/           🆕 独立 TTS 包 (DashScope CosyVoice/qwen3-tts + OpenAI 适配器 + RealtimeModel + ModelCard YAML)
          messagebus/   分布式消息总线 (LocalBus + RedisBus) + CoordBus 四原语 (Lock/Registry/Queue/Log) + TeamBus + SessionProjection, 对齐 #1849
记忆层     memory/        ReMe (文件/向量) + 7 向量后端 (5完整+SQLiteVec+2占位) + Hybrid Search(BM25+Reranker) + Dream 演化 + 知识图谱 + FTS5
可观测性   observability/ OpenTelemetry + LangSmith + Langfuse + TracingMiddlewareAdapter(语义属性提取) + otelbridge
          logging/       结构化日志规范 (stdlib slog 封装 + 环境配置 + 请求级 context logger)
演化层     evolver/       GEP Gene/Capsule 类型 + Evolver 客户端 + Run/Reflect/Solidify 流程 + Skill2GEP 蒸馏
扩展层     plugin/        🆕 Plugin 系统: Plugin 接口 + Manager + Registrar + YAML 配置 + .so 加载 (Linux)
辅助包     config/        配置管理
          credential/    凭证抽象/Factory/Schemas
          dist/          分发/打包
          loader/        文档加载器 (TextLoader/DirLoader)
          output/        结构化输出 (StructuredRunner + 校验重试)
           retry/         重试策略 (线性退避/永久错误分类)
          async/         异步任务执行池
          plan/          PlanNotebook 多步骤任务管理
          logging/       结构化日志规范 (stdlib slog 封装 + LOG_LEVEL/LOG_FORMAT 环境配置 + 请求级 FromContext)
           skill/         SkillBox + SkillViewer + load_skill + 蒸馏到 Gene
           tests/         跨语言契约测试
           embedding/onnx/ ONNX HTTP 代理：CLIP/Whisper 预处理(Go本地) + 嵌入推理(HTTP代理) + 模型管理器
           a2a/           A2A 增强：认证/限流/WebSocket/安全中间件/ShardRouter/ClusterManager
           benchmark/     性能基准测试目录 + Catalog (Gateway/Memory/Plan/Graph/A2A)
平台层     channel/       多平台集成 (ChannelEvent/Channel接口/Gateway/Dispatcher/Routing + Webhook 通道)
            hub/           Hub 市场 (MCP/Skill 卡片浏览+安装 + FSHub + zip-slip 防护)
 治理层     controlplane/  LoopX 风格长时序治理平面 (Goal/Quota/Gate/Evidence/Lease/Reward/Kanban + 演化闭环)
终端层     console/       bubbletea TUI 终端 (HITL y/n/a 确认 + 中断 + 三档事件渲染)
```

## 核心模块与代码量（非测试行 / 测试行 / 测试文件数）

| 模块 | 非测试行 | 测试行 | 测试文件 | 说明 |
|------|----------|--------|----------|------|
| `memory/` | 9,780 | 4,470 | 51 | 最大模块: ReMe + 7 向量后端 (5完整+SQLiteVec+2占位) + Dream + Summarizer/Compactor + Hybrid Search |
| `gateway/` | 5,168 | 4,134 | 29 | HTTP/SSE/WS/流式 HTTP + AG-UI + Tool Offload + 调度 CRUD |
| `tool/` | 4,628 | 2,848 | 27 | file/shell/web/json/multimodal + Task/Schedule/Subagent + A2A |
| `tool/a2a/` | 198 | 208 | 1 | A2A 分布式 ReAct: A2ATool 委托子任务 + Registry 多 Agent 管理 |
| `agent/` | 3,610 | 3,146 | 18 | V1/V2 接口 + Base + ReActAgent + ReplyStream + StructuredOutput |
| `model/` | 2,642 | 2,467 | 20 | 10+ 后端 + Responses API + 35 ModelCard + TTS/Audio + Router |
| `workspace/` | 5,200 | 3,800 | 16 | Local/Docker/E2B/**K8s**/**Bubblewrap**/**Daytona**/**OpenSandbox** + MCP Gateway + Offloader (7 后端) |
| `toolkit/` | 1,492 | 1,187 | 12 | 工具注册/执行 + MCP 适配 + MCP Prompts/Resources/Sampling |
| `service/` | 3,300 | 2,000 | 9 | Storage(Auth/Cipher) + **SQLStorage (SQLite)** + MemoryStorage + RedisStorage + Access Policy |
| `permission/` | 1,205 | 661 | 8 | 规则引擎 + Bash AST |
| `formatter/` | 946 | 1,203 | 7 | 3 独立 Formatter (OpenAI/Anthropic/Gemini) + 2 别名 + 11 MultiAgent 变体 |
| `rag/document/` | 85 | 60 | 2 | Section/Chunk 数据模型 (对齐 Python rag._document) |
| `rag/parser/` | 1,100 | 600 | 8 | Parser 接口 + Registry + Text/PDF/PPTX/Image/**Word/Excel** 解析器 (6 parser) |
| `rag/chunker/` | 165 | 195 | 2 | Chunker 接口 + ApproxTokenChunker (近似 token, rune 边界) |
| `rag/blob/` | 90 | 110 | 2 | BlobStore 接口 + LocalBlobStore |
| `rag/kb/` | 330 | 300 | 2 | KnowledgeBase 句柄 + KBManager + InMemoryVectorStore (多租户) |
| `rag/index/` | 145 | 210 | 2 | Worker 全管道编排 + Queue channel 调度 |
| `middleware/`(含RAG/Agentic/Injection) | 2,000 | 1,400 | 9 | 洋葱链 7 钩子 + Budget/TTS/LongTermMemory/RAG/AgenticMemory/**Injection** |
| `logging/` | 115 | 120 | 1 | 结构化日志规范 (slog 封装 + 环境配置 + FromContext 请求级) |
| `skill/` | 1,038 | 393 | 4 | SkillBox + SkillViewer + load_skill |
| `evolver/` | 928 | 94 | 1 | GEP Gene/Capsule + Evolver 接口(16方法) + Mock/MCP/Recording 客户端 |
| `messagebus/` | 230 | 175 | 1 | 🆕 分布式消息总线 (LocalBus + RedisBus pub/sub, 对齐 #1849) |
| `event/` | 1,007 | 434 | 4 | 20+ 事件类型 + Bus |
| `a2a/` | 1,652 | 1,156 | 7 | Agent 间协议 + 认证/限流/WebSocket + 安全中间件 + ShardRouter/ClusterManager (Go 独有) |
| `plugin/` | 480 | 520 | 3 | 🆕 Plugin 系统: Plugin 接口 + Manager + Registrar + YAML 配置 + .so 加载 (Linux) |
| `benchmark/` | 120 | 80 | 1 | 🆕 性能基准测试目录 + Catalog |
| `message/` | 734 | 641 | 3 | Msg + 多模态 ContentBlock |
| `plan/` | 612 | 459 | 3 | PlanNotebook 多步骤管理 |
| `hook/` | 524 | 364 | 4 | 经典 Hook + StreamHook + Trace Exporter |
| `credential/` | 510 | 94 | 1 | 凭证抽象/Factory |
| `observability/` | 477 | 514 | 4 | OTel + LangSmith + TracingMiddlewareAdapter |
| `state/` | 384 | 305 | 3 | AgentState 持久化 (JSONFile/Redis) |
| `middleware/` | 1090 | 723 | 5 | 洋葱模型中间件链 + BudgetControl (token 预算) + TTSMiddleware + LongTermMemory (mem0 三模式) |
| `tts/` | 398 | 158 | 1 | 🆕 独立 TTS 包：Model/RealtimeModel + DashScope + OpenAI 适配器 + ModelCard YAML (5 卡片) |
| `embedding/` | 603 | 240 | 2 | 5 后端 (OpenAI/Ollama/Gemini/DashScope/DashScope多模态) + FileCache + ModelCard YAML (7 卡片) |
| `embedding/onnx` | 2,400 | 1,200 | 2 | ONNX HTTP 代理：CLIP/Whisper 预处理 + 模型管理器 + 跨模态相似度 |
| `rag/` | 301 | 259 | 4 | RAG + Tika 集成 |
| `session/` | 250 | 131 | 2 | 会话管理 |
| `workflow/` | 245 | 245 | 2 | MapReduce/Condition/Loop/Parallel |
| `dist/` | 244 | 167 | 1 | 分发/打包 |
| `async/` | 191 | 176 | 1 | 异步执行池 |
| `pipeline/` | 130 | 239 | 3 | 顺序/并行流水线 |
| `schedule/` | 121 | 160 | 1 | Cron 调度器 |
| `output/` | 104 | 129 | 1 | 结构化输出 |
| `loader/` | 99 | 71 | 1 | 文档加载器 |
| `tests/` | 95 | 69 | 1 | 跨语言契约测试 |
| `config/` | 91 | 53 | 1 | 配置管理 |
| `msghub/` | 87 | 105 | 1 | 消息中心广播 |
| `reflection/` | 66 | 143 | 1 | 反思机制 (Writer+Critic) |
| `retry/` | 61 | 95 | 1 | 重试策略 |
| `shutdown/` | 42 | 35 | 1 | 优雅关闭 |
| `interruption/` | 51 | 52 | 1 | 中断处理 |
| `runcontext/` | 39 | 37 | 2 | 运行时上下文 |
| `controlplane/` | 4,089 | 1,453 | 3 | LoopX 风格治理平面: Goal/Quota/Gate/Evidence/Lease/Reward/Kanban + SQL 持久化 + 演化闭环 |
| `console/` | 491 | 184 | 1 | bubbletea TUI 终端: 多轮对话 + HITL 确认 + 中断 + 三档事件渲染 |
| **总计** | **~71,100** | **~41,600** | **~331** | 165 包，持续增长 |

## 测试

```bash
go test ./... -race -count=1   # 全量通过（提交前强制）
```

推荐使用 `make test`（见根目录 Makefile）或 `make ci` 进行本地模拟。

全项目 `go test ./...` 和 `go build ./...` 均通过，无已知编译错误。

## 编码规范（更新于 P0/P1 工程化）

- **所有包必须通过** `go test ./... -race -count=1`
- **提交前必须** `gofmt -l .` 返回空（或使用 `make fmt` / `make fmt-check`）
- 优先使用 `golang.org/x/sync/errgroup` 进行并发控制
- 中断检查优先使用原子操作，配合 `sync.RWMutex` 保护复杂状态
- 多模态结果使用 `message.ContentBlock` 子类型封装
- 工具返回值使用 `tool.Response` 规范类型
- 事件流使用 `<-chan event.AgentEvent` channel 模式
- Agent 状态挂起/恢复通过 `InjectEvent()` + `pendingExternalEvents` 实现
- 推荐安装 golangci-lint 并通过 `make lint` / `golangci-lint run ./...`
- 新代码优先使用顶级 `embedding/` 包（NewOpenAI / NewOllama / NewGemini / NewDashScope + WithFileCache）。`memory/embedding` 仅为向后兼容的 adapter（已标记 Deprecated）。
- 中间件使用洋葱模型（`OnXxx(ctx, agent, input, next XxxNext) -> (*Msg, error)`），支持 Reply/Reasoning/Acting/ModelCall/SystemPrompt 五个拦截点
- **结构化日志**：新代码用 `logging` 包（基于 stdlib `log/slog`）。`logging.Default()` 获取 logger，`logging.FromContext(ctx)` 取请求级 logger，`logging.WithLogger(ctx, l)` 注入。键值对用 `logging.KeyAgentID`/`KeySessionID` 等常量。环境变量 `LOG_LEVEL`/`LOG_FORMAT` 配置。禁止 `fmt.Println`/`log.Printf` 做日志（仅 examples/demo 允许）。

本地推荐流程（使用 Makefile）：
```bash
make fmt
make vet
make lint   # 如已安装 golangci-lint
make build
make test   # 或 make ci
```

## 关键设计决策

1. **事件驱动范式**：从"消息为中心"转向"事件为中心"，Agent 输出为 `ReplyStream() -> <-chan event.AgentEvent`
2. **状态机模型**：Agent 可挂起/恢复，`AgentState` 可序列化到 Redis/JSONFile，支持跨请求恢复
3. **Channel vs Iterator**：使用 Go channel 替代 Python AsyncGenerator，背压自然处理
4. **struct embedding 复用**：`agent.Base` 通过 embedding 提供统一生命周期（钩子、中断、关闭）
5. **Formatter 解耦**：消息格式化与模型实现分离，通过 interface 注入。3 个独立 Formatter (OpenAI/Anthropic/Gemini) 均实现 `Formatter` 接口，DashScope/Ollama 为 OpenAI 别名
6. **Workspace 沙箱**：工具执行隔离在 Local/Docker/E2B 环境中，通过 `workspace.Workspace` 注入
7. **可观测性对齐**：TracingMiddlewareAdapter 实现完整 middleware 5 接口，支持 agent 生命周期 tracing（on_reply/on_reasoning/on_acting/on_model_call/on_system_prompt），结合 TracedAgent + OTel/LangSmith + otelbridge 自动桥接
8. **GEP 自演化对齐 (Phase 6)**：通过 evolver/ 包引入 Gene/Capsule 类型、Evolver 接口（16 方法）、高层 GEPFlow（RunAndSolidify 闭环）、skill2gep 蒸馏、Mock + MCP + Recording 客户端。利用现有 gateway MCP 网关 + ReMe + a2a 实现"轻量桥接"
9. **泛型工具构造**：`NewFunctionToolAuto[T]` 通过 Go 泛型自动从 handler 签名提取 JSON Schema 并解码输入
10. **洋葱模型中间件**：`middleware.Chain` 按类型自动分类并构建拦截链，Reply/Reasoning/Acting/ModelCall 为洋葱递归闭包，SystemPrompt 为管道顺序执行
11. **OpenAI Responses API 独立后端**：`model/openai_response/` 直接使用 `net/http`（不依赖 SDK），支持推理模型的链式思考事件流
12. **多模态路由**：`model.MultimodalRouter` 根据输入媒体内容自动在文本/视觉模型间切换
13. **流式 HTTP 传输**：受 MCP 2025-03-26 启发，`gateway/streamable_http.go` 单一端点支持 POST/GET/DELETE，含 SSE 流式、AG-UI 转换和会话回放
14. **Tool Offload 机制**：长时间运行工具从同步 ReAct 循环中卸载，完成后通过提示注入方式通知 Agent
15. **V1/V2 ReAct 共享逻辑**：`agent/react/react_shared.go` 提取 PreCall/BeforeModel/AfterModel/CheckFinalAnswer 等共享生命周期方法，V1 `replyInternal` 与 V2 `replyStreamInternal` 共用同一套 hook 处理逻辑
16. **Builder 模式统一**：所有 `model/` 子包均提供 `NewBuilder()` 作为规范入口（与 `Builder()` 向后兼容）
17. **断路器保护**：`model/circuit_breaker.go` 实现三状态 (Closed/Open/HalfOpen) 断路器，通过 `WithCircuitBreaker(threshold, cooldown)` 启用
18. **流式结构化输出**：`output.StructuredRunner.RunStream()` 支持 ChatStream 增量 JSON 解析，在流式传输中实时返回部分解析结果
19. **增量上下文压缩**：`CompressContext` 增强：预截断超大工具结果 → 结构化摘要合并（去重累积）→ 摘要超限时 LLM 元压缩。`CompressionWatermark` 追踪累积压缩量
20. **A2A 分布式 ReAct**：`tool/a2a/` 包将 `a2a.Client` 包装为 `tool.Tool`，ReAct Agent 可自动委托子任务给远程 Agent。支持同步/流式双模式 + `Registry` 多 Agent 注册/发现
21. **DAG 计划执行器**：`plan/dag_executor.go` 使用 Kahn 算法实现拓扑排序，独立步骤并行执行 + 重试策略 + 回调钩子 + 依赖结果传递。`ValidateDAG`/`ReadySteps` 辅助方法
22. **MCP 扩展能力**：`toolkit/mcp/capabilities.go` 添加 Prompts（`PromptsClient`）、Resources（`ResourcesClient`）、Sampling（`SamplingClient`）三个可选接口 + SDKClient 实现 + 类型安全 helper 函数
23. **SQLiteVec 向量后端**：`memory/vector/sqlite_vec_store.go` 基于 `modernc.org/sqlite` + `sqlite-vec` 的纯 Go 持久化向量存储，零 CGO 依赖，`vec0` 虚拟表 + 归一化向量 L2→余弦相似度转换 + SQL 元数据过滤
24. **知识图谱推理 + 抽取**：`memory/graph/reasoning.go` 提供 FindAllPaths/MultiHopNeighbors/Subgraph/HasCycle/NodeImportance/SearchNodes 算法；`memory/graph/knowledge_extractor.go` 使用 LLM 从文本提取实体/关系三元组并自动注入图谱
25. **Plugin 系统**：`plugin/` 包提供 `Plugin` 接口（Init/Register/Shutdown 三阶段生命周期）+ `Manager`（优先级排序 + 配置驱动）+ `Registrar`（Model/Tool/Memory/Hook/Middleware/Formatter 六注册点）+ YAML 配置 + Linux `.so` 动态加载（build tag 隔离）
26. **性能工程化**：`gateway/pprof.go` 提供 pprof 端点集成（`WithPProf()` 链式启用）；`benchmark/` 包统管全项目基准测试 Catalog；Makefile `bench-save`/`bench-compare`/`bench-cpu`/`bench-mem` 目标支持基线对比和性能剖析
27. **中间件请求变更生效**（对齐 Python v2）：`agent/react/stream.go` 的 `runModel`/`invokeModelChat` final 闭包从 `input` 读取 `Messages`/`ChatOpts`，使 on_reasoning/on_model_call 中间件对请求的变更（注入 hint、强制 tool_choice）真正生效。`BudgetControlMiddleware` 据此实现：累加器经共享 `context.Context` 在 on_reply/on_model_call/on_reasoning 三链间传递，超限注入 HintBlock + 强制 `tool_choice=none`
28. **独立 TTS 抽象**（对齐 Python #1832）：`tts/` 包定义轻量 `Model`/`RealtimeModel` 接口（独立于重型 `model.AudioModel`），DashScope 后端（CosyVoice/qwen3-tts 同步生成 API）+ OpenAI 适配器（复用 `model.OpenAITTS`）+ 内嵌 YAML ModelCard。`TTSMiddleware` 合成回复语音并以 base64 `AudioBlock` 附加（默认非致命、Strict 可选）
29. **长期记忆中间件**（对齐 Python #1775）：`middleware.LongTermMemory` 接口（`Search`/`Add`）+ `LongTermMemoryMiddleware` 三模式（static_control 在 on_reasoning 注入检索 HintBlock 并回写 / agent_control 暴露 `search_memory`/`add_memory` 工具 / both）。`NewFuncLongTermMemory` 函数适配器零耦合桥接 ReMe 向量记忆或外部 mem0 HTTP；用户隔离（UserID/AgentID namespacing）
30. **自定义 Agent 类注册表**（对齐 Python #1838）：`AgentFactory.RegisterAgentClass(name, AgentClassBuilder)` 支持非 ReAct agent 类型；`service.AgentConfig.AgentClass`（默认 `"react"`）+ `SubagentTemplates` 字段；`Build`/`BuildFromTyped` 经 `buildAgentForClass` 分发，未注册类清晰报错。向后兼容（默认行为不变）
31. **Embedding ModelCard 化**（对齐 Python #1852）：`embedding.ModelCard` + 内嵌 per-provider YAML 卡片 + `ListModelCards`/`ModelCardsByProvider`，与 chat 模型卡片体系一致，支撑 Studio 动态表单与 provider 自文档化
32. **Agent Team 运行时 spawn + 权限继承**（对齐 #1833/#1815）：`AgentFactory.BuildSubagentTools` 在 `BuildSessionAgent` 中将 leader 的 `SubagentTemplates` spawn 为 `SubagentTool`；子 agent 继承 leader 的 `*permission.Engine` 并共享会话工作区 + 基础 file/shell 工具集（不含嵌套 subagent 工具，避免递归）
33. **分布式消息总线**（对齐 #1849）：`messagebus.Bus` 抽象 + `LocalBus`（进程内、发布非阻塞）+ `RedisBus`（Redis pub/sub、多进程）；`AppConfig.MessageBus`/`Server.WithMessageBus` 接入，支撑跨进程 cancel/wake-up/tool-offload-complete 协调
34. **Agent Team 异步协作运行时**（对齐 Python agentscope AgentTeam）：完整移植 Python 的 leader/worker 异步协作模型（区别于 #32 的同步 agent-as-tool）。`messagebus.TeamBus` 接口（`InboxPush`/`InboxDrain`/`EnqueueWakeup`/`SubscribeWakeup`）由 `LocalBus`（内存）与 `RedisBus`（Redis LIST+BLPOP，持久不丢）实现；`service.Team`/`TeamMember` 数据模型 + Storage CRUD；四个 team 工具 `TeamCreate`/`AgentCreate`/`TeamSay`/`TeamDelete`（`gateway/team_tools.go`，工具调用时自查前置条件，权限恒 ALLOW）；`WakeupDispatcher`（`gateway/wakeup_dispatcher.go`）订阅 wakeup 信号 → drain inbox → 重组 `<team-message>` 为 user 输入 → `SessionManager.Run` 重跑空闲 worker，忙时轮询重试。worker 为独立 `source=team` agent+session，跨进程靠共享 storage( inbox)+bus(wakeup) 协调。`BuildSessionAgent` 按 `AgentConfig.Source` 挂载 leader 全集/worker 仅 `TeamSay`；`Server.Start` 自动拉起 dispatcher
35. **RAG 托管知识库管道**（对齐 Python `rag/`+`app/rag/`+`middleware/_rag.py`）：`rag/document`(Section/Chunk)→`rag/parser`(Registry 按 MIME 路由: Text/PDF/PPTX/Image，PDF 纯 Go `ledongthuc/pdf`+panic recover，PPTX 纯 stdlib `archive/zip`+`encoding/xml`，Image 内容嗅 sniff)→`rag/chunker`(ApproxTokenChunker，utf8/4 近似 token+rune 边界切分)→`rag/blob`(LocalBlobStore `local://` URI)→`rag/kb`(KnowledgeBase 句柄绑定 embedder+collection+多租户 metadata_filter 强制隔离；KBManager+CollectionPerKB；InMemoryVectorStore 余弦+filter+doc聚合)→`rag/index`(Worker 串联全管道+Queue channel 背压)。`RAGMiddleware`(static/agent/both 三模式：OnReply 检索→context state→OnReasoning 注入 HintBlock；search_knowledge 工具；MinScore 过滤；错误不中断)。KB HTTP 路由 8 端点(CRUD+multipart/JSON上传+搜索+octet-stream扩展名嗅探兜底)。用 Go 地道方式补齐 Python 在 RAG 托管服务上 90% 的差距
36. **CoordBus 消息总线协调原语**（对齐 Python `app/message_bus` 通用原语）：在 `messagebus` 包新增 `CoordBus` **可选接口**（`Lock`/`Registry`/`Queue`/`Log`），不破坏现有 `Bus`/`TeamBus`。LocalBus：channel 信号量锁+`time.AfterFunc` TTL 自动释放、slice+notify channel ctx 可取消队列、map 注册表；RedisBus：`SET NX PX`+Lua token 释放锁、HASH 注册表、`RPUSH`+`BLPOP` FIFO 队列、LIST 游标日志。`AsCoordBus` 类型断言获取，nil 时所有消费者降级单进程。`CoordKeys` 集中管理业务键格式（QueueName/LockKey/RegistryNS/LogNS/ProjectionNS + WakeupKind wake/resume）
37. **跨会话投影**（对齐 Python session_projection）：`gateway.SessionProjection` 基于 CoordBus registry，一个会话可向另一会话投影 UI 卡片（如 worker HITL 请求投影到 leader）。无 CoordBus 时优雅降级 no-op。HTTP `GET/DELETE /api/v1/sessions/{id}/projections`
38. **零构建现代 Web UI 控制台**（对齐 Python React Web UI，用 Go 地道方式）：`examples/web_ui/static/` 零 npm/Node 构建依赖——原生 vanilla JS + SSE + `go:embed`，单二进制部署。侧边栏导航三视图（Chat AG-UI SSE 流式 / Knowledge Bases CRUD+上传+检索 / System health+models）。GitHub 风深色主题（CSS 变量+响应式卡片网格）。前端直连 gateway HTTP API 无代理层。AG-UI 协议与 Python React 前端兼容
39. **Agentic Memory 自主记忆**（对齐 Python `AgenticMemoryMiddleware`）：`middleware.AgenticMemoryMiddleware` 文件式自主记忆——agent **自己用已有文件工具**(Write/Read/Edit)管理 Markdown 记忆文件，区别于 ReMe(被动检索)和 LongTermMemoryMiddleware(暴露 search/add 工具)。`LocalMemoryStore`(EnsureLayout/ReadMemoryMD/ListFiles+frontmatter 解析)。OnSystemPrompt 注入记忆指令+有界 MEMORY.md 快照(truncateApproxTokens utf8/4)；OnReply 启动异步检索 goroutine；OnReasoning `chan string`+`select{default:}` 非阻塞轮询注入 HintBlock 仅一次。`FileSelector` 闭包解耦(默认 KeywordSelector 零依赖，LLM 选择器 `WithSelector` 注入)，不硬依赖 model 包。4 类记忆(user/feedback/project/reference)+不该存什么+frontmatter 格式指令
40. **Channel 多平台集成**（对齐 Python `app/channel/` #1997）：`channel/` 包把 Agent 接入外部消息平台（Discord/飞书/Webhook）。`ChannelEvent` 归一化入站消息（channel_id/user_id/chat_id/content）→ `Channel` 接口（Start 长连接+emit/SendText/Close，与 gateway 解耦无 import cycle）→ `Gateway`（Router.Resolve→Runner.RunUserTurn，错误不杀 listener）→ `Dispatcher`（goroutine 生命周期）。`RouteTable`/`Binding` 路由（exact>prefix>default），session 派生 `<prefix><chat_id>`（per-chat 分组）。`WebhookChannel` 零依赖 HTTP 通道（POST→事件，实测闭环）。`gateway.ChannelRunner` 适配 AgentRegistry+SessionManager（异步运行+收集事件流文本→SendText 回发+inflight 去重）。Gateway HTTP：GET /api/v1/channels + POST /{id}/webhook。Discord/飞书适配器待 `discordgo` 依赖
41. **Hub 市场**（对齐 Python Hub #2197）：`hub/` 包提供 MCP/Skill 市场。`Hub` 接口（ListMCPCards/ListSkillCards + 游标分页）+ 泛型 `Page`/`FilterCards`。`InstallMCPs` 复用 `toolkit/mcp.ConnectServers`（缺失二进制优雅跳过）；`InstallSkill` HTTP 下载 + zip/tar/tar.gz 解压 + **zip-slip/tar-slip 跨平台防护**（safeJoin 拒绝绝对路径+`..`+Windows/POSIX 双检查）+ 64MiB 下载/解压双上限。`builtin.FSHub`（mcps.json+skills.json 目录即市场）。Gateway：GET /api/v1/hubs + 浏览/安装 5 路由
40. **中间件 7 钩子扩展**（对齐 Python `on_check_permission`+`on_compress_context`，#c613a860）：`middleware.PermissionInterceptor`+`CompressionInterceptor` 新增两个洋葱模型拦截点。`PermissionResult` 本地类型（避免循环依赖：middleware→permission→test→skill→toolkit→agent→middleware）。`Chain.Classify` 自动分类新接口；`ChainPermission`/`ChainCompression` 构建洋葱链。中间件可拦截/替换/绕过权限决策，可控制/跳过上下文压缩。向后兼容（不实现新接口则跳过）
41. **运行时状态注入**（对齐 Python `InjectionConfig`，#29792cd9）：`middleware.InjectionMiddleware` 实现自动时间感知——`InjectionConfig`(timezone/time_format/time_interval/context_buffer_ratio/extra_fields/template)。OnSystemPrompt 追加 Runtime Awareness 指令；OnReasoning 按 TimeInterval 间隔注入 `<system-reminder>` HintBlock（含当前时间+extra_fields XML 标签）。sync.Mutex 保护 lastInjectAt。Go `time.LoadLocation` 支持任意时区。默认 30 分钟间隔，模板 `{runtime_state}` 占位符
42. **PowerShell 专用工具**（对齐 Python `PowerShell`，#798bc181）：`tool/shell/powershell.go` 自动探测 `pwsh`（优先）或 `powershell.exe`（`exec.LookPath`+`sync.Once` 缓存）。命令经 UTF-16-LE Base64 编码（`unicode/utf16.Encode`+`encoding/base64`）传给 `-EncodedCommand`，避免转义问题。`-NoLogo -NoProfile -NonInteractive` 标志。输出上限 30,000 字符（`truncOutput`+截断标记）、超时上限 600s（默认 120s）。所有命令非 ReadOnly。Windows 优先，非 Windows 优雅报错
43. **资源跨用户共享策略**（对齐 Python `ResourceAccessPolicy`，#1aeb03d0）：`service/access/policy.go` 定义 `ResourceKind`(Credential/Agent/KB)+`ResourcePermission`(READ/EDIT)+`ResourceRef`+`Policy` 接口（`ListAccessible`/`CanEdit`）。`DenyAllPolicy` 默认拒绝所有跨用户访问（owner 总能编辑自己的资源）。`StaticPolicy` 内存策略用于测试/小部署。策略不管理用户/组/成员关系，只映射 viewer→资源引用；应用子类化以从 config/IAM/LDAP 读取规则
44. **Word/Excel 解析器**（对齐 Python `WordParser`/`ExcelParser`，#811425c0/#e67e54f5）：纯 Go `archive/zip`+`encoding/xml` 解析 OOXML。WordParser：`word/document.xml` 状态机遍历 `<w:p>`/`<w:r>`/`<w:t>` 段落+`<w:tbl>`/`<w:tr>`/`<w:tc>` 表格（Markdown pipe-table 渲染+`|` 转义）。ExcelParser：`xl/sharedStrings.xml` 共享字符串解析+`xl/worksheets/sheetN.xml` cell 解析（`t="s"` 类型索引解析+数字直接取值）+workbook.xml sheet 名映射+Markdown table 渲染+等宽 padding。Registry 已注册 6 个 parser（Text/PDF/PPTX/Image/Word/Excel）
45. **SQL 存储后端**（对齐 Python `AsyncSQLAlchemyStorage`，#b49a26b9）：`service.SQLStorage` 基于 `database/sql`+`modernc.org/sqlite`（纯 Go 零 CGO）实现完整 `Storage` 接口。8 张表（users/sessions/agents/credentials/messages/snapshots/schedules/teams）各有索引列+JSON payload 列。原子 upsert 使用 SQLite `ON CONFLICT(conflictCol) DO UPDATE SET`（支持自定义冲突列如 snapshots 的 session_id）。级联删除事务（删除 user → 级联 sessions→messages/snapshots + agents + credentials + schedules + teams）。WAL 模式提升并发。泛型 `scanRows[T]` 消除重复代码。`:memory:` 模式零配置测试
46. **Workspace 7 后端扩展**（对齐 Python Workspace 多元化，#81538d35/#7af58b11/#dd71a372/#15b5243e）：从 3 个后端扩展到 7 个。**K8sWorkspace** 使用 kubectl CLI（非 client-go，保持二进制轻量）——`kubectl run` 创建 Pod+`kubectl wait` 等 Ready+`kubectl exec` 执行命令/文件操作+`kubectl delete --force` 清理。支持包装已存在 Pod（`NewK8sWorkspaceForExistingPod`）。**BubblewrapWorkspace** Linux 用户命名空间沙箱——文件操作直接在宿主机 BaseDir（bind-mount 到 /workspace）+Execute 每次生成新鲜 bwrap 进程（`--ro-bind` 系统目录+`--bind` 工作目录+`--unshare-all/net`+`--die-with-parent`）。**DaytonaWorkspace/OpenSandboxWorkspace** REST API 客户端——POST 创建+toolbox/files API 文件操作+execute API 命令执行+Bearer 认证+httptest mock 测试。全部复用 `cmdRunner` 抽象（K8s）或 `http.Client`（REST），保持可测性
47. **bubbletea TUI Console**（对齐 Python `agentscope.console` #2297）：`console/` 包——bubbletea+bubbles+lipgloss 三态机（idle/running/confirming）。ReplyStream 事件经 `waitForEvent` tea.Cmd 桥接；三档 verbosity（quiet/default/debug）渲染文本/思考/工具/令牌；HITL 逐工具 y/n/a 确认→`InjectEvent(NewUserConfirmResult)` 恢复，Ctrl+C 拒绝全部；running 时 Ctrl+C→`agent.Interrupt()`（类型断言）；无会话持久化（对齐 Python）。`examples/console/` 演示 calculator 工具 + ModeDefault 权限的完整确认流
48. **治理-演化闭环 + steer/abort 端点**：controlplane 目标迁移到 completed（仅 `handleUpdateCPGoal` 一处）→ 触发 `evolver.Solidify`（DecisionSource=controlplane/PrimaryCause=goal_id，`AppConfig.AutoSolidifyOnGoalComplete` opt-in，异步+recover）。`POST /v2/sessions/{id}/steer|interrupt` 端点挂 `SessionManager` 活动 run（`Steer` 类型断言，interrupt 复用 Terminate）。**多租户会话隔离**：`checkSessionAccess` 校验 session 归属（storage 可用时，跨用户 POST/GET/DELETE/WS 握手全部 404，不泄露存在性）。react 中断恢复文案附带部分回复摘要（PyV2 #2209）
49. **KB 可观测性 + 工具增强**（对标 PyV2 #2372/#2114/#2378）：`VectorStore.ListChunks`（doc_id+filter 聚合、chunk_index 排序、vector 剥离）+ `GET /kb/{id}/documents/{doc}/chunks|raw` 端点（Worker 索引时把 `blob_uri` 注入 chunk metadata 实现 blob 溯源）+ KB 列表 documents/chunks 计数富化 + web_ui 分块浏览/原文链接。read_file 对非有效 UTF-8 内容返回 base64 `DataBlock`（http.DetectContentType 嗅探 media_type + 说明文本块）；`tool.WithInputSchema(schema)` 覆盖自动生成的参数 schema（typed handler 不受影响）。核查确认 #2366（host OS 选 shell）在 Go 架构下不存在——shell 选择由各 Workspace 后端自决（Docker/K8s 容器内 `sh -c`，Local 即宿主）
50. **on_reply 续循环 + Team 失败通知 + 治理指标**：`middleware.ErrContinueReply` 哨兵（OnReply 返回即请求再跑一轮，对齐 PyV2 #2322 吞 ReplyEndEvent）+ `Base.Call` 有界续循环（中间回复回灌 `ReplyInput.Messages`，3 轮上界防死循环，超界保留最后一轮）。worker turn 失败 → `watchRunFailure` 检测 ErrorEvent → `<team-error>` 推入 leader inbox + wakeup（对齐 #2386）。`observability.ControlPlaneCollector` 把 Kernel 9 个治理计数器导出为 `agentscope_controlplane_*_total` Prometheus counter（scrape 拉快照零 goroutine）+ `Server.WithMetricsRegistry` 自动接线 `/metrics`

## 已知代码质量问题（审阅发现，待修复）

| # | 模块 | 问题 | 严重度 |
|---|------|------|--------|
| 1 | `memory/graph/` | ~~`DeleteNode` 删除节点后边未清理~~ → 已修复 | ✅ 已修复 |
| 2 | `memory/` | ~~`MemoryType` 常量不一致~~ → 已统一到 vector 包 | ✅ 已修复 |
| 3 | `formatter/` | ~~Anthropic/Gemini Formatter 不实现 `Formatter` 接口~~ → 已统一签名，全部实现 | ✅ 已修复 |
| 4 | `model/` | ~~Router circuit-breaker 虚假描述~~ → 已修正注释 | ✅ 已修复 |
| 5 | `a2a/` | ~~AuthMiddleware JWT 手动 base64 解码，无过期检查/算法验证~~ → 已改用 `golang-jwt/jwt/v5`，支持 HS256/384/512 + 过期 + claims 提取 | ✅ 已修复 |
| 6 | `a2a/` | ~~WebSocket CheckOrigin 允许所有来源~~ → 已改为可配置 | ✅ 已修复 |
| 7 | `rag/` | ~~`sortScores` 冒泡排序 O(n²)~~ → 已改 sort.Slice | ✅ 已修复 |
| 8 | `credential/` | ~~10 个 Type 常量但仅 3 个 provider 有实现~~ → 已补齐全部 10 个 provider (DashScope/DeepSeek/Moonshot/xAI/Ollama/OpenAIResp/vLLM) | ✅ 已修复 |
| 9 | `agent/` | ~~`agent.AgentState` 与 `react.AgentState` 同名冲突~~ → 已改名 ConfigSnapshot | ✅ 已修复 |
| 10 | `memory/vector/` | ~~ES/Pgvector 占位桩静默 no-op~~ → 已改为返回 ErrNotImplemented + 新增 SQLiteVec 完整实现作为替代 | ✅ 已修复 |
| 11 | `embedding/` | ~~Gemini Dimensions() 硬编码 768~~ → 已改为可配置 | ✅ 已修复 |
| 12 | `agent/react/` | ~~ReActAgent 缺少 V2Agent 编译断言~~ → 已添加 | ✅ 已修复 |
