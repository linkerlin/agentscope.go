务必使用中文进行思考、推理和输出！
======

# AgentScope Go 开发备忘录

## 项目概述

本项目是 [AgentScope](https://github.com/agentscope-ai/agentscope) 的 Go 语言实现，采用地道的 Go 惯用法构建生产级 AI Agent 框架。当前版本 **v2.3.0**。

## V2 架构总览

```
网关层     gateway/       HTTP/SSE/WebSocket/流式 HTTP + AG-UI Protocol + 多租户认证 + Tool Offload
          a2a/           Agent-to-Agent 协议 (Go 领先 PyV2)
服务层     service/       Storage 抽象 + CRUD + AES-GCM 加密 + Cipher
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
          middleware/     Agent 生命周期中间件链 (洋葱模型: Reply/Reasoning/Acting/ModelCall/SystemPrompt)
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
记忆层     memory/        ReMe (文件/向量) + 7 向量后端 (5完整+SQLiteVec+2占位) + Hybrid Search(BM25+Reranker) + Dream 演化 + 知识图谱 + FTS5
可观测性   observability/ OpenTelemetry + LangSmith + TracingMiddlewareAdapter + otelbridge
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
          rag/           RAG 集成 (含 Apache Tika + Memory Adapter)
          skill/         SkillBox + SkillViewer + load_skill + 蒸馏到 Gene
          tests/         跨语言契约测试
           embedding/onnx/ ONNX HTTP 代理：CLIP/Whisper 预处理(Go本地) + 嵌入推理(HTTP代理) + 模型管理器
           a2a/           A2A 增强：认证/限流/WebSocket/安全中间件/ShardRouter/ClusterManager
           benchmark/     性能基准测试目录 + Catalog (Gateway/Memory/Plan/Graph/A2A)
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
| `workspace/` | 1,911 | 1,223 | 9 | Local/Docker/E2B + MCP Gateway |
| `toolkit/` | 1,492 | 1,187 | 12 | 工具注册/执行 + MCP 适配 + MCP Prompts/Resources/Sampling |
| `service/` | 1,336 | 1,177 | 6 | Storage/Auth/Cipher |
| `permission/` | 1,205 | 661 | 8 | 规则引擎 + Bash AST |
| `formatter/` | 946 | 1,203 | 7 | 3 独立 Formatter (OpenAI/Anthropic/Gemini) + 2 别名 + 11 MultiAgent 变体 |
| `skill/` | 1,038 | 393 | 4 | SkillBox + SkillViewer + load_skill |
| `evolver/` | 928 | 94 | 1 | GEP Gene/Capsule + Evolver 接口(16方法) + Mock/MCP/Recording 客户端 |
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
| `middleware/` | 318 | 126 | 2 | 洋葱模型中间件链 |
| `embedding/` | 316 | 85 | 1 | 5 后端 (OpenAI/Ollama/Gemini/DashScope/DashScope多模态) + FileCache |
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
| **总计** | **~43,000** | **~28,500** | **~250** | 持续增长 |

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
