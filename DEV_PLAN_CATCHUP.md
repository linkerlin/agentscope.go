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

**Phase 5 继续研发** (当前)：
- Tracing middleware 对齐：增强 TracingMiddlewareAdapter (支持更多 interceptor)，在 full_service 示例中集成 TracedAgent 演示。
- 测试：embedding/memory/gateway/observability 测试通过，新增 tracing adapter 测试。
- 示例：full_service 使用 tracedBase + tracing。
- 文档：全面更新 (README, CHANGELOG, AGENTS, tutorial, deployment, DEV_PLAN) 突出 Phase 5 tracing。
- 验证：full build + targeted tests 通过。
下一步: 更多 e2e (app bootstrap, studio), 完善 tracing middleware 实现 (添加更多 hook), version bump, 发布准备。

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
