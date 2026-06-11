# AgentScope Go 追赶 Python 参考实现的研发计划 (Catch-up Plan)

(完整计划已生成并保存在会话 plan 文件中。此为项目内可见副本。)

**目标**：使 `agentscope.go` 在“生产级 Agent Service 交付体验”和“配套抽象完整性”上达到或接近 Python `agentscope` (v2) 的水平。

**核心滞后点总结** (来自详细对比)：
1. 生产级 Web UI / Studio（最大可见差距）：Python 有完整 React Studio，Go 只有极简 demo。
2. 统一 App/Service 框架易用性：Python `create_app` + lifespan 自动管理；Go 有强大组件但高层 bootstrap 不够“一键”。
3. Credential 抽象：Python 有 typed CredentialBase + Factory + list_schemas() (用于动态表单)；Go 是扁平 provider+encrypted。
4. 其他：独立 embedding 包、丰富 Docker 模板、数据模型丰富度、端到端 Studio 示例。

**Go 已有优势** (保留并发扬)：ReMe 记忆、事件驱动 V2 + Hook、a2a/workflow 等高级编排、强 gateway (AG-UI / offload / schedule / MCP)、优秀测试。

**总体策略**：
- 增量演进，复用现有 gateway/service/agent_factory 等。
- Go 地道实现 (struct、interface、最小依赖)。
- UI 优先自包含 Go 方案 (HTMX + templates) + 保证 AG-UI 协议完整（可复用 Python 前端）。
- 每阶段可验证：`go test ./... -race` 全绿 + 可运行示例 + 测试覆盖。

## 推荐实施阶段 (已用户批准)

### Phase 1: 基础抽象对齐 & 高层 Bootstrap 强化
- 新 `credential/` 包 (base + factory + 具体 provider + schemas 支持)。
- 丰富 service 实体 (AgentData、ScheduleData、ChatModelConfig 等对齐)。
- 强化 AppConfig / NewApp + 启动生命周期骨架 (schedule restore、默认装配)。
- 新增 `/credential/schemas` 端点。
- 交付：credential 包 + 测试 + 更新 production 示例 + schemas 端点。

### Phase 2: Service 完整性 & 生命周期
- 完善启动恢复 schedules。
- 标准工具集便捷注册 helper。
- ToolOffload 等通过高层配置自动启用。
- 丰富 CRUD 对齐更多字段。

### Phase 3: Workspace/Tool Polish + Embedding 包
- 多 Dockerfile 模板 (pypi/src/node 等)。
- 新顶级 `embedding/` 包 (base + providers + file cache)。
- 工具/ meta 补充。

### Phase 4: Rich Web UI / Studio (最大差距)
- 推荐：`examples/studio` 或 `studio/` 轻量 Go UI (html/template + HTMX + Tailwind CDN 或 templ)。
- 核心功能：Auth、Agents CRUD、Credentials (schemas 驱动表单)、Sessions+Chat (复用现有流)、Schedules。
- 辅助：完善 AG-UI 文档，支持直接用 Python 的 TS Studio 连接 Go gateway。
- 目标：一键启动示例后浏览器内完成注册→创建 cred+agent→聊天→schedule。

### Phase 5: 追踪、文档、测试、发布 (当前进行中)
- **Tracing middleware 对齐**：将现有 observability 包装成易用 middleware，参考 Python `_tracing/`。
- **文档全面更新**：README、CHANGELOG、AGENTS.md、docs/*、DEV_PLAN_CATCHUP.md（突出 auto-assembly、embedding、Studio、provider parity）。
- **测试补充**：新 embedding providers、auto bootstrap e2e、Studio 流程、保持 `-race` 全绿。
- **发布准备**：增强示例、更新 CHANGELOG、version notes、迁移指南。
- **可选**：evaluation / finetune / voice 后续对齐。

**验证标准**：每阶段测试全绿、可演示示例、文档更新、无破坏性变更（或有迁移路径）。

**当前状态**：Phase 1-4 核心已完成（auto bootstrap + credential + embedding + Studio 轻量 + provider + 迁移）。Phase 5 文档先行，然后 observability / 测试 / 发布。

更多细节见会话生成的完整 plan.md。

---

## 实施进度记录

**2026-06-11 更新（按“选项1”继续 Phase 1/2 收尾）**：

- **Schedule restore on startup**：
  - `gateway/app.go`：`NewApp` 当 `Storage` 存在时自动创建 `BackgroundTaskManager` 并注入 storage（内部已实现 `loadPersistedSchedules` + `ListAllSchedules`）。
  - `gateway/server.go`：新增 `Start()`（启动 BTM cron + 恢复计划）和 `Close()`（停止后台）。
  - 这使得使用 `NewApp` + `RegisterAppRoutes` + `srv.Start()` 后，持久化的 schedule 在进程重启后自动恢复执行（与 Python lifespan + SchedulerManager.restore 行为对齐）。

- **默认/标准工具集 helper**：
  - 新增 `gateway/standard_tools.go` + 测试：`StandardTools(opts)` 一键返回常用工具（file.RegisterAll + Task* + Schedule* + web fetch + json parse/query + extra）。
  - 便于在 bootstrap 或 agent 创建时统一注入。

- **生产示例升级**：
  - `examples/production/main.go` 改为使用 `gateway.NewApp(AppConfig{...})` + `RegisterAppRoutes` + `srv.Start()` + `defer srv.Close()` + `gateway.StandardTools(...)`。
  - 明确注释 schedule 持久化 + 恢复能力。

- **验证**：
  - `go test ./gateway -race` 通过。
  - `go build ./examples/production` 成功。
  - 相关变更最小且向后兼容。

下一步可继续扩展 Studio（Phase 4）或进一步丰富自动工具注入 / 动态 agent + schedule 集成示例。

**最新完成（响应“添加更多自动装配”具体请求 + 继续研发）**：
- 初始 static agent 现在也在 production/full_service 示例中“默认”使用 auto tools（通过在启用 AutoStandardTools 时用相同的 StandardToolsOptions 构建 base agent，与动态 session agent 一致）。
- StandardTools 内部在 IncludeTask 且 TaskStore 未提供时自动创建简单的 in-memory TaskStore（state.NewTaskStore）。NewApp auto 路径也显式确保 deps.TaskStore 被设置。
- examples/production 进一步精简（移除冗余 credential demo 部分）。
- 新建 examples/full_service ：极简但功能完整的示例， heavily 依赖 auto-assembly（一行 AppConfig 获得几乎全部能力），并有详细注释说明 static + dynamic 都受益。
- 扩展 Studio 示例：
  - studio/main.go 启动时使用 NewApp + AutoStandardTools + WorkspaceBaseDir + AutoToolOffload，使 Studio 服务器自身演示自动装配。
  - index.html 大幅增强，新增醒目的“自动装配效果演示”卡片，列出具体自动获得的工具（file/glob/grep/web/json + auto TaskStore 的 Task* + schedule + offload + 权限 + workspace + schedule restore）。
  - 新增 agents.html、schedules.html、chat.html 页面，使用 HTMX 调用现有 API，展示创建 agent 后 session 自动拥有丰富工具。
  - 添加 demo register 按钮，方便无 JWT 流程测试。
- Phase 3 开头：workspace/docker.go 增强 RenderDockerfile 支持多个 flavor（pypi、src、node、full），匹配 Python 的多个 Dockerfile 模板。更新了测试。
- **standalone embedding/ 包 + 更多 provider + 迁移 + gateway 增强**（Phase 3 完成）：
  - 新增 Gemini (HTTP embedContent) 和 DashScope (OpenAI compat, 支持 multimodal model 提示) providers。
  - memory/embedding/openai.go 和 local.go 已迁移/包装到使用新 embedding/ 包内部实现，减少重复（保留兼容字段/AsModel）。
  - gateway/app.go + NewApp 增强：支持 AppConfig.EmbeddingCacheDir ，自动用 embedding.WithFileCache 包装传入的 EmbeddingModel。
  - embedding 包测试 + 整体 build/test 通过。
- **Phase 4 Studio 进一步打磨**：
  - chat.html 升级为真实 SSE streaming from /v2/chat/stream ，使用 fetch ReadableStream 解析 data: JSON 事件。
  - 解析 text deltas 实时追加，特殊处理 tool_call_start / tool_result 事件，在 UI 中高亮显示 “[AUTO TOOL] xxx started” 和 “[AUTO TOOL RESULT] ...” ，直接演示 auto-assembled tools (file, task, web, json 等) 的实际调用结果。
  - agents.html / schedules.html 升级为 demo 友好的 JS fetch (带 X-Demo-User header)，完整 create/list 流程。
  - index.html 增强 demo register 脚本，设置全局 htmx headers，支持后续 CRUD 演示。
  - 所有页面突出 auto tools + schedule restore + workspace 等效果。
- 更新了 full_service/studio 配置以启用 embedding cache 演示。
- DEV_PLAN_CATCHUP.md 已同步所有进展。
- `go build ./...` + race tests (embedding, memory/embedding, gateway, studio 等) 全部通过。

Phase 3/4 按请求大幅推进并进一步完善：
- Providers polish: Gemini/DashScope 添加多模态注释、额外构造测试、错误处理提示；embedding 包文档增强。
- memory/embedding 迁移后清理：wrapper 保留测试兼容，核心委托新包，测试通过，重复减少。
- Gateway embedding: AppConfig.EmbeddingCacheDir 自动包装新包 cache；full_service 实际启用带 cache 的 embedding；handler 利用传入 cached model。
- Studio polish 继续：
  - agents/schedules: JS fetch + 漂亮表格 (ID/Name/Actions)，添加 delete 按钮，"Use in Chat" 联动设置 chat agent_id (完整 CRUD + 集成)。
  - chat: SSE 实时 + tool 事件高亮显示实际 auto tools 调用结果；studio demo agent 也 attach StandardTools。
  - demo auth: register 设置 header，列表/CRUD 自动用 X-Demo-User。
  - 所有 UI 强调 auto 装配效果 + 工具结果可见。
- full_service/studio 配置启用 embedding cache 演示。
- 验证: build + race tests 通过；DEV_PLAN 更新。
下一步: 更多多模态实现、e2e 测试脚本、Phase 5 文档/发布。
  - 打印和 UI 都突出 auto 效果。

**Phase 5 继续研发** (当前，继续)：
- Tracing middleware 对齐：TracingMiddlewareAdapter 完整实现 middleware 接口，可直接 .Middlewares() 使用。添加 RecordingTracer 用于可见演示 spans。
- 在 full_service 和 studio 中实际使用 tracingMW + RecordingTracer 演示，打印 recorded spans。
- 测试补充：新增 TestRecordingTracerWithTracedAgent；在 gateway/app_test 增强 bootstrap tracing 测试；observability 测试通过。
- 示例/文档：full_service/studio 打印 tracing demo spans；更新 index.html 列表 tracing；全面完善 README（tracing 章节）、AGENTS.md（架构）、tutorial/deployment/concepts/index（tracing/embedding 扩展）；DEV_PLAN/CHANGELOG 同步。
- 验证：build + tests OK。
下一步: 更多 e2e (结合真实 OTel + schedule + studio 流程), 发布 prep (full test, notes, migration guide)。Phase 5 追踪/测试/示例/文档基本完成，可视为追赶工作阶段性完成。
- 新增 TestTracingAdapterImplementsMiddlewareInterfaces 验证 chain 集成。
- 构建/测试验证通过 (observability, gateway)。
- 文档/计划更新记录本次完善。
所有 Phase 5 tracing 子项已推进。构建 OK。

**“添加更多自动装配”增强（最新迭代）**：
- AppConfig 扩展支持更多“一键”选项：
  - `WorkspaceBaseDir` → 自动 `NewWorkspaceManager`（会话工作区沙箱）。
  - `AutoStandardTools` + `StandardToolsOptions` → 自动为动态 per-session ReAct agents 装配标准工具集（通过 `SessionAgentDeps.ExtraTools` 注入 `sessionWorkspaceTools`）。
  - `AutoToolOffload` → 自动连通 ToolOffloadManager 到 BTM 和 session deps。
  - `DefaultPermissionMode` → 自动配置权限引擎。
- `NewApp` 内部自动：
  - 创建 WorkspaceManager（基于 base dir）。
  - 构建并存储 `defaultSessionDeps`。
  - 在 `buildSessionAgentFromStorage` / `resolveAgentForRequest` 中 merge 默认 deps（工具、offload、权限、schedule mgr）。
- `Server` 新增 `defaultSessionDeps` 字段 + 访问器，支持运行时自动装配。
- `examples/production/main.go` 演示使用这些字段实现更简洁的丰富配置。
- 效果：用户只需在 AppConfig 里设置几个布尔/路径字段，即可获得 workspace + 标准工具 + tool offload + schedule restore + 权限 等自动能力，极大接近 Python `create_app(storage, workspace_manager=...)` 的体验。
- 测试与构建验证通过。

---

## Phase 6: Evolver GEP 自演化优势对齐 (当前阶段)

**目标**：将 ./evolver/ (及 evolver.py) 的核心优势引入 agentscope.go ，同时保持 Go 轻量、事件驱动、MCP 友好、不引入重依赖。重点不是全量 port，而是**类型+协议+高层流程对齐 + 桥接现有能力**（ReMe 记忆 + a2a + gateway MCP + skill + tracing）。

**evolver 核心优势 (来自源码 schemas + MCP 工具 + README + 论文)**：
- **Genes vs Skills**：紧凑、可复用、带 signals_match + strategy + constraints + validation + routing_hint 的“策略基因”，而非冗长 ad-hoc 文档式 skill。研究证明在演化中信号更强、迭代更好（arXiv:2604.15097）。
- **Capsules**：成功演化的快照（带 blast_radius、outcome、execution_trace、derivation_tokens、a2a eligible）。
- **GEP Pipeline**：evolver_run（信号提取+基因选择+GEP提示生成）→ evolver_reflect（风险检测）→ evolver_solidify（验证、记事件、更新基因、存 capsule，支持 dryRun、human_intervention、reused_asset、git 回滚语义）。
- **Typed Memory**：remember/recall/reflect（gene/capsule/event 类型）、narrativeMemory + memoryGraph（vs 扁平检索）。
- **Meetings**：结构化多代理演化会议（research/code/debug/grokteam 等，带 proceed/human_input/finalize/playback/audit）。
- **ATP / Hub**：fetch/claim/complete 任务 + sync_to_hub + 资产复用审查（A2A 协议增强）。
- **Skill <-> GEP**：skill2gep / skillDistiller / skillPublisher / autoDistill，可将既有 skill 蒸馏成 gene 并发布。
- **Safety & Audit**：safety_status、事件列表、policy/guard、rollback 保障。
- **Proxy/Mailbox 解耦**：与 agentscope.go a2a/ 及 gateway 天然互补。

**Go 已有互补优势**（继续发扬）：
- ReMe 记忆系统（混合检索、向量化 summarizer、window/compactor、5+ 向量后端）已非常成熟，可作为 gene/capsule 存储的强载体。
- a2a 协议（Go 领先 PyV2）。
- gateway 完整 MCP 暴露（session_mcp_gateway）—— agent 可直接调用 evolver MCP 工具。
- 事件驱动 + middleware + Phase5 tracing（RecordingEvolver 类似 RecordingTracer）。
- 轻量 HTMX Studio + auto-assembly。

**实施策略**：
- 新 `evolver/` 包（types + client interface + mock/recording + high-level GEP flow）。
- 最小侵入集成：skill.DistillToGene、memory 增加 evolution 记忆类型标签、a2a 文档提及 ATP 模式、gateway AppConfig 增加 EvolverEnabled 提示。
- 示例 + Mock 先行，真实后端通过 MCP 桥接（零额外二进制依赖）。
- 文档先行同步（中英）。

**已交付（本次迭代）**：
- `evolver/types.go`：Gene / Capsule / Task / BlastRadius / Outcome 等完整结构体（JSON 标签 + Create/Validate 对齐 evolver/src/gep/schemas/* + seed + 实时 MCP 数据）。支持 category=repair/optimize/innovate/explore，constraints、routing_hint、tool_policy、epigenetic、source 等。
- `evolver/client.go`：Evolver 接口（覆盖 list_genes/upsert、run/reflect/solidify、remember/recall、meeting_*、fetch/claim/complete、stats/safety）。提供 NewMockEvolver（预加载真实基因样本）、RecordingEvolver（调用追踪，对齐 Phase5）。
- `evolver/gep.go`：NewGEPFlow + RunAndSolidify（run→reflect→solidify 完整演示闭环）、DistillSkillToGene（skill2gep 风格启发式蒸馏）。
- `evolver/evolver_test.go`：类型验证、Mock GEP 流程、distill、recording 全覆盖，-race 通过。
- `skill/skill.go`：AgentSkill.DistillToGene 委托（零拷贝集成）。
- `memory/reme_types.go`：新增 MemoryTypeGene / Capsule / EvoEvent（与 narrativeMemory 对齐，可直接存取）。
- `gateway/app.go`：AppConfig.EvolverEnabled 注释 + 建议。
- `examples/evolver/main.go`：完整可运行 demo（GEP 闭环、distill、recall、meeting、recording calls 打印、与 ReMe 结合说明）。
- 验证：`go build ./...` + `go test ./evolver -race` + `go run ./examples/evolver` 全部通过，输出清晰展示 calls、gene 选择、capsule 固化、distilled gene 等。

**使用示例（代码即文档）**：
```go
flow := evolver.NewGEPFlow(evolver.NewMockEvolver())
// 或真实：通过 gateway MCP 让 agent 工具调用 evolver__evolver_run 等
res, sol, _ := flow.RunAndSolidify(ctx, evolver.RunConfig{Context: "timeout recurring", Strategy: "repair-only"}, false)
gene := evolver.CreateGene(...) ; flow.Client.UpsertGene(ctx, gene)
```

**下一步（持续）**：
- 丰富真实 MCP client 实现（当前示例用 Mock；用户可基于 agentscope toolkit.MCPClient 快速包装 evolver__* 工具名调用）。
- a2a 增强：引入 ATP Task 拾取/心跳/claim 模式（复用现有 a2a 注册）。
- Studio / gateway 端点：可选暴露 /evolver/genes、/capsules 列表（HTMX 表格），或在 chat 中高亮 “GEP gene applied”。
- ReMe 深度桥接：narrativeMemory / memoryGraph 的 Go 实现或 adapter（当前先用类型标签 + remember 语义）。
- 固化安全：集成 gitOps / workspace rollback 钩子（参考 evolver gitOps）。
- 更多端到端：full_service / studio 里演示“自愈 agent”（遇到错误自动触发 GEP）。
- 跨语言 fixture：tests/cross_lang 增加 gene/capsule JSON。
- 发布：version rc.3，迁移指南中加入 “如何用 evolver MCP 给你的 Agent 加上自进化”。

**验证标准**：pkg 测试全绿、example 可独立运行并打印有意义 GEP 资产、文档更新、与现有 ReMe/a2a/gateway 无冲突。

此阶段标志“追赶 Python 参考 + 超越 ad-hoc”进入“对齐工业级自演化引擎”新阶段。
