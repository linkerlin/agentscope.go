# AgentScope.Go 演进实施 TODO

> 来源：`演进方案.md`（v3）+ `演进方案v4.md` + `演进方案v5.md` + `演进方案v6.md`
> 状态：**Phase 5-15 全部完成**（v2.6.0 发布准备就绪；Phase 15 于 2026-08-22 收官）
> 更新日期：2026-08-22（Phase 15：KB 可观测性 / 工具增强 / on_reply 续循环 / Team 失败通知 / 治理指标导出 / 性能快照）
>
> **代码规模**（2026-07-30 审阅实测）：~66,500 非测试行 / ~39,900 测试行 / 303 测试文件 / 162 包 / 45 示例 / `go build ./...` + `go vet ./...` 全绿
>
> **Phase 5-8 已完成（v2.4.0）**：
> - **Phase 5 RAG 托管知识库**：rag/{document,parser(4),chunker,blob,kb,index} + RAGMiddleware + KB HTTP API + examples/rag_kb
> - **Phase 6 消息总线深化**：CoordBus 四原语(Lock/Registry/Queue/Log) Local+Redis + 跨会话投影
> - **Phase 7 现代 Web UI**：零构建 SPA 控制台(Chat/KB/System) + go:embed
> - **Phase 8 Agentic Memory + Tracing**：文件式自主记忆 + Span 语义属性提取 + otelSpan 桥接
>
> **v3 新增差距（Python 06-25 至 07-30，67 commits）**：
> - Workspace 沙箱：Python 3→8 后端（新增 K8s/Daytona/Bubblewrap/Apple Container/OpenSandbox）
> - 存储后端：Python 新增 SQLAlchemy 异步存储（Postgres/MySQL/SQLite）
> - 中间件钩子：Python 5→7 钩子（新增 on_check_permission/on_compress_context）
> - 运行时注入：Python 新增 InjectionConfig（自动时间/任务/上下文注入）
> - RAG 解析器：Python 4→6（新增 Word/Excel）
> - 向量库：Python 新增 MongoDB/Elasticsearch 完整实现
> - 工具：Python 新增专用 PowerShell 工具
> - 资源共享：Python 新增 ResourceAccessPolicy 跨用户共享框架
>
> **Phase 9-11 路线**：Workspace 沙箱扩展(P9) → 存储深化+中间件精细化(P10) → RAG 扩展+向量库补全(P11)（**已全部完成**）
>
> **Phase 12-13 已完成（v4 路线，2026-08-08）**：
> - **Phase 12 Channel 多平台集成**：`channel/` 核心（Event/Channel/Gateway/Dispatcher/Routing）+ **Webhook + Discord + 飞书 三平台** + gateway 接入（ChannelRunner/HTTP 路由）+ 飞书 agent 工具（send_message/list_chats）+ 3 示例（16+5+10+7 测试）
> - **Phase 13 Hub 市场**：`hub/`（卡片/Hub 接口/安装器 zip-slip 防护）+ FSHub + gateway 5 路由 + examples/hub_demo（12+5+6 测试）
>
> **守护任务已完成**：Plugin 示例（examples/plugin_demo + docs/PLUGIN.md）
>
> **仅剩 maintainer 执行**：`git tag v2.5.0 && git push --tags && gh release create v2.5.0 -F RELEASE_NOTES_v2.5.0.md`

---

## 实施原则

1. **每批次改动后必须执行**：`go build ./...` + `go test ./... -race -count=1 -timeout=12m`
2. **本轮聚焦 Phase 5-8**（托管服务化补齐）；旧 Phase 1-4 未完成项保留在末尾
3. **补齐不复制**：用 Go 地道方式实现（纯 Go 库、`embed`、interface 组合、channel 背压）
4. **守住护城河**：每 Phase 须含"放大既有优势"任务，A2A/ReMe/evolver/Plugin 禁止稀释
5. **最小依赖**：parser 优先纯 Go 库，blob 首版只 Local，避免一上来铺全家桶
6. **示例即文档**：每个新增模块必须可独立运行/测试 + 配 README

---

## Phase 5：RAG 托管知识库服务（P0，目标 2026-Q3）⚡本轮最高优先级

> 对标 Python `rag/` + `app/rag/` + `middleware/_rag.py`。Go 现仅 Tika 适配 + 155 字节桩，缺整条管道 90%。

### 5.1 文档模型与解析器
- [x] `rag/document/document.go`：Document/Section/Chunk 数据模型（含 metadata slide_index/media_type），替换 155 字节桩
- [x] `rag/parser/parser.go`：`Parser` 接口 + `SupportedMediaTypes()` + 注册表 `Registry`（按 mediaType 路由）
- [x] `rag/parser/text.go`：TextParser（纯 stdlib）
- [x] `rag/parser/pdf.go`：PDFParser（基于 `ledongthuc/pdf`，纯 Go 无 CGO，含 panic recover）
- [x] `rag/parser/pptx.go`：PPTXParser（纯 stdlib `archive/zip` + `encoding/xml`，逐页 `<a:t>` 提取，自然排序）
- [x] `rag/parser/image.go`：ImageParser（内容嗅探 → DataBlock，可选 OCR hook）
- [x] 各 parser 单测（TextParser/PPTX/Image 含正例；PDF 含错误路径；端到端管道测试）

### 5.2 分块器
- [x] `rag/chunker/chunker.go`：`Chunker` 接口 `Chunk([]Section) ([]Chunk, error)`
- [x] `rag/chunker/approx_token.go`：ApproxTokenChunker（`len(utf8)/4` 近似 token，chunk_size/overlap 可配，DataBlock 透传，rune 边界安全切分）
- [x] 分块器单测（短文本/多块/多字节/DataBlock透传/顺序索引/零值默认/空输入）

### 5.3 Blob 存储
- [x] `rag/blob/blob.go`：`BlobStore` 接口 Put/Get/Delete
- [x] `rag/blob/blob.go`：`LocalBlobStore`（基于 `os`，自动建目录，`local://` URI 方案）
- [ ] `rag/blob/s3.go`：S3BlobStore（`aws-sdk-go-v2`，可选，首版 Local 已验收）

### 5.4 知识库管理器
- [x] `rag/kb/kb.go`：`KnowledgeBase` 运行时句柄（绑定 embedding+vectorstore，懒建表，search/insert/delete/list，metadata_filter 多租户隔离）
- [x] `rag/kb/kb.go`：`KBManager` + `CollectionPerKB` 实现 + `Spec`/`EmbedderFactory`
- [x] `rag/kb/inmemory_store.go`：`InMemoryVectorStore`（余弦相似度 + filter + doc 聚合，生产用远程后端替换）
- [x] KB 单测（增删查/多租户隔离/重复创建/缺失/删表/embedder 错误/空文本跳过）
- [ ] `rag/kb/dimension.go`：DimensionPolicy 维度校验（可选，CollectionPerKB 已允许任意维度）

### 5.5 索引工作流
- [x] `rag/index/worker.go`：`Worker`（blob→parse→chunk→embed→insert 全管道 + OnStatus 回调）
- [x] `rag/index/worker.go`：`Queue`（channel 调度，自然背压，SubmitCtx 可取消）
- [ ] `rag/index/sweeper.go`：IndexSweeper（失败任务重试，需持久任务态，ponytail 暂缓）
- [x] 端到端测试：上传→索引→可检索（Worker 正例 + 缺KB/缺blob/不支持类型 错误路径 + Queue 排空/满）

### 5.6 RAG 中间件
- [x] `middleware/rag.go`：RAGMiddleware（static/agent/both 三模式；OnReply 检索→OnReasoning 注入 HintBlock；search_knowledge 工具；MinScore 过滤）
- [x] `middleware/rag_test.go`：static 注入/agent 无注入/空查询/错误吞没/MinScore/工具/系统提示词 全覆盖

### 5.7 KB HTTP 路由
- [x] `gateway/kb_handlers.go`：KBService 聚合 + RegisterKBRoutes（8 端点：CRUD + 文档上传 multipart/JSON + 文档列表/删除 + 搜索）
- [x] `gateway/server.go`：`kbService` 字段 + `WithKBService()` 链式选项
- [x] `gateway/app.go`：AppConfig.KBService + RegisterAppRoutes 自动注册
- [x] `gateway/kb_handlers_test.go`：创建/列表/删除/上传(JSON+multipart)/搜索/文档删除/404/空查询 端到端 HTTP 测试

### 5.8 示例与文档
- [x] `examples/rag_kb/`：上传文档→索引→提问，端到端 demo（离线 stub embedder）+ README
- [x] `docs/RAG.md`：托管 KB 教程

---

> **Phase 5 状态：✅ 完成**（5.1-5.8 全部落地）
> 可选暂缓项：S3 BlobStore、IndexSweeper（需持久任务态）、DimensionPolicy（CollectionPerKB 已允许任意维度）

---

## Phase 6：消息总线深化 + 服务编排层（P1，目标 2026-Q4）

### 6.1 消息总线原语补齐
- [x] `messagebus/coord.go` + `coord_redis.go`：`CoordBus` 可选接口（Lock/Registry/Queue/Log）+ `AsCoordBus` 类型断言
- [x] `Lock(ctx,key,ttl)` 分布式锁（LocalBus channel 信号量 + TTL 自动释放；RedisBus SET NX PX + Lua token 释放 + 轮询）
- [x] `RegistrySet/Get/List/Delete` 注册表（LocalBus map；RedisBus HASH）
- [x] `QueuePush/QueuePop` 通用队列（LocalBus slice+notify ctx 可取消；RedisBus RPUSH+BLPOP FIFO）
- [x] `LogAppend/LogRead` 追加日志（LocalBus slice；RedisBus LIST + 游标分页）
- [x] `messagebus/keys.go`：业务键约定（QueueName/LockKey/RegistryNS/LogNS/ProjectionNS + WakeupKind wake/resume）
- [x] 四原语 Local+Redis 双后端测试（含并发互斥序列化、TTL、ctx 取消、miniredis）

### 6.2 跨会话投影
- [x] `gateway/projection.go`：SessionProjection（基于 CoordBus registry，无 CoordBus 时优雅降级 no-op）
- [x] HTTP 路由：`GET/DELETE /api/v1/sessions/{id}/projections[/{key}]` + RegisterProjectionRoutes 自动注册
- [x] 投影测试（LocalBus 正常/no-op 降级/HTTP CRUD/无 bus 404）

### 6.3 服务编排层
- [x] `gateway/app.go` `CreateApp` 工厂（`NewApp` + `AppConfig` + `RegisterAppRoutes`，已对齐 Python create_app）
- [ ] ChatService 统一编排提炼（现有 wakeup_dispatcher + background_task + team_tools 已实现编排，提炼为独立 ChatService 类型为可选优化）
- [ ] `gateway/middleware/inbox_middleware.go`：团队消息排空注入（现有 wakeup_dispatcher 已在 dispatcher 层实现 inbox drain → user 输入重跑，agent 中间件层为可选补充）
- [ ] `gateway/middleware/state_change_middleware.go`：状态变更广播（可选）
- [ ] `gateway/middleware/tool_offload_middleware.go`：提炼现有 tool_offload 为标准中间件（可选）

---

## Phase 7：现代 Web UI（P2，目标 2027-Q1）

### 7.1 零构建 SPA 控制台（方案 A，Go 原生）
- [x] `examples/web_ui/static/index.html`：控制台外壳（侧边栏导航 + Chat/KB/System 三视图 + 模态框）
- [x] `examples/web_ui/static/styles.css`：现代 GitHub 风深色主题（CSS 变量、响应式卡片网格）
- [x] `examples/web_ui/static/app.js`：全部前端逻辑（路由 + AG-UI SSE chat + KB CRUD/上传/检索 + system）
- [x] `go:embed static/*` 内嵌进单二进制，**零 npm/Node 构建依赖**
- [x] 端到端验证：index 200 / KB create / 文档上传索引 / 语义检索 全通过
- [x] `examples/web_ui/main.go`：接入 KBService（stub embedder 离线）+ 注册 V2/KB/Models/Projection 路由

### 7.2 Python Studio 兼容（方案 B）
- [x] AG-UI 协议（`/v2/chat?protocol=agui`）与 Python React 前端兼容；文档给出"Python 前端 + Go 后端"部署方案

### 7.3 Studio 深化
- [x] `examples/web_ui` 控制台增加知识库管理 + 文档上传 + 检索面板（对应 Phase 5 KB 路由）
- [ ] 现有 `examples/studio` 服务端模板增加：知识库页 + 索引状态监控（可选，web_ui 控制台已覆盖）

---

## Phase 8：记忆范式 / 可观测 / 存储 / 工具补全（P3，目标 2027-Q1）

### 8.1 Agentic Memory
- [x] `middleware/agentic_memory.go`：AgenticMemoryMiddleware（文件式自主记忆；OnSystemPrompt 注入指令+MEMORY.md 快照；OnReply 启动异步检索 goroutine；OnReasoning 注入 HintBlock 仅一次）
- [x] `LocalMemoryStore`：文件系统后端（EnsureLayout/ReadMemoryMD/ListFiles + frontmatter 解析）
- [x] `FileSelector` + `KeywordSelector`（默认关键词选择器，可 LLM 替换；`WithSelector`）
- [x] `truncateApproxTokens`：MEMORY.md 快照有界（utf8/4 近似 token，rune 边界安全）
- [x] 区分 LongTermMemoryMiddleware（被动/工具）vs Agentic（agent 自主用文件工具）
- [x] `examples/agentic_memory/`：离线可运行 demo + README
- [x] 单测：store/frontmatter/keyword/truncation/系统提示词注入/异步检索注入/无匹配不注入/仅注入一次/nil 降级

### 8.2 Tracing 提取深化
- [x] `observability/otel.go`：`Span` 接口扩展 `SetAttributes` + `SpanAttr` 类型 + 属性 helper（String/Int/Int64/Float64/Bool）+ `SpanStatus`
- [x] `observability/wrap.go`：`TracingMiddlewareAdapter` 五钩点提取语义属性（reply.message_count/last_role · reasoning.iteration · tool.name/input_keys · model.name/message_count · system_prompt.length）+ `RecordingSpan`（捕获属性供测试）+ `RecordingTracer.LastSpan/SpanByName`
- [x] `observability/provider.go`：`otelSpan.SetAttributes` 桥接 → 真实 OTel span（属性流入生产 tracer）
- [x] 测试：五钩点属性提取 / RecordingSpan 查找 / noopSpan / 属性 helper

### 8.3 存储投影与索引态
- [x] （KB 索引态由 `rag/kb.InMemoryVectorStore` 自管；session projection 由 `messagebus.CoordBus` registry 自管；调度态由 `gateway/background_task` 自管）——各子系统已有独立持久化路径，无需在 service.Storage 重复投影
- [ ] 远程向量库（Qdrant/Milvus）持久化 KB 索引态（可选，生产规模时接入）

### 8.4 Task 工具 CRUD 补全
- [x] `tool/task/`：TaskCreate/Get/List/Update 四工具 + status 状态机 + BlockedBy 依赖 + AddBlockRelation（**已有实现**，测试通过）

### 8.5 预设权限模式
- [x] `permission/`：5 种预设模式 ModeDefault/Explore/AcceptEdits/Bypass/DontAsk（**已有实现**，比 Python EXPLORE/STANDARD/AUTONOMOUS 更完整；`NewEngine(ModeExplore, rules)` 即一键预设）

---

## 守护任务（贯穿，放大既有优势）

- [x] A2A：文档 `docs/A2A.md` + 示例（a2a/a2a_redis_registry/a2a_secure）已有
- [x] evolver：文档 `docs/EVOLVER.md` + 示例 + MCPEvolver 真实后端已有
- [x] ONNX：文档 `docs/ONNX.md` + 示例已有
- [x] Plugin：`examples/plugin_demo/`（三阶段生命周期 + YAML + 工具注册，实测运行）+ `docs/PLUGIN.md`

---

## 旧 Phase 1-4 未完成项（保留）

### Phase 1
- [ ] 创建 Git tag `v2.0.0` 与 GitHub Release（需 maintainer 执行）
- [ ] 为 `gateway/` 增加多租户 SSE/WS 集成测试
- [ ] 提升 `memory/`、`gateway/` 外其他模块的覆盖率
- [ ] 标记 integration tests 为 `//go:build integration`

### Phase 2
- [ ] 验证 Python Studio UI 可连接 Go Gateway（Phase 7 方案 B 替代）
- [ ] 录制视频（可选）

### Phase 3
- [x] 维护 `agentscope-go-mcp-servers` 列表 + filesystem/web-search/browser/github MCP 配置示例（`toolkit/mcp/catalog.go` ServerSpec + CommonServers 目录 6 server + YAML 加载 + `docs/MCP.md` + `examples/mcp_servers/` 可运行 demo）
- [x] OIDC/OAuth2 SSO（`gateway/oidc_handlers.go` 已有）+ RBAC 角色权限（`service/rbac.go`：3 角色 + 细粒度权限 + `RBACMiddleware` + 防提权 `VerifyRoleAssignment` + `UserRole.OrgID` 组织隔离字段 + `AuditLogger`/`MemoryAuditLogger`，**已补 9 测试**）
- [x] Langfuse 接入（`observability/langfuse.go` + `langfuse_observer.go`：HTTP 批量 ingestion + Basic auth + trace/span/generation 事件映射 + 批量缓冲 flush，对齐 LangSmith 模式）
- [x] structured logging（slog）规范：`logging/` 包（stdlib slog 封装 + LOG_LEVEL/LOG_FORMAT 环境配置 + FromContext/WithLogger 请求级 + 键常量 + Discard）+ gateway `RequestLoggingMiddleware`（request_id/user_id/method/path/status/duration 结构化访问日志）。全量迁移既有 249 处 log.Print 为增量后续，examples 允许保留。
- [x] AuditLogger 接入 gateway handler（`gateway/audit.go`：requireAuth 内置审计，自动记录 who/method/path/status；`GET /api/v1/audit-logs` admin-only 查询；AppConfig.AuditLogger；MemoryAuditLogger 加锁线程安全）

### Phase 4
- [ ] NATS 后端支持（可选）
- [ ] K8s Operator（可选）
- [ ] evolver 真实 MCP 后端集成（移至"守护任务"）
- [ ] 生产级自愈 Agent 示例（移至"守护任务"）

---

## Phase 9：Workspace 沙箱扩展（P0，目标 2026-Q4）⚡本轮最高优先级

> 对标 Python 最近 5 周新增的 5 个 Workspace 后端。Go 现有 3 个(Local/Docker/E2B)，需扩到 8 个。

### 9.1 Kubernetes Workspace
- [x] `workspace/k8s.go`：K8sWorkspace（kubectl CLI 模式，无需 client-go 重依赖）
- [x] Pod 生命周期管理（kubectl run/wait/delete）
- [x] 文件操作（kubectl exec cat/tee/ls/stat/mkdir）
- [x] 命令执行（kubectl exec -- sh -c）
- [x] 支持自定义 namespace/image/labels
- [x] 支持包装已存在的 Pod（NewK8sWorkspaceForExistingPod）
- [x] 测试：文件操作/ListDir/Stat(文件+目录)/Execute/WorkingDir/错误退出码/Close/现有Pod/缺Image/fmtPerm

### 9.2 Bubblewrap Workspace
- [x] `workspace/bubblewrap.go`：BubblewrapWorkspace（Linux bwrap 用户命名空间沙箱）
- [x] 文件操作直接在宿主机 BaseDir 上（bind-mount 到沙箱 /workspace）
- [x] Execute 通过 bwrap --ro-bind/--bind/--proc/--dev/--tmpfs/--unshare-all 构建
- [x] 支持自定义只读挂载/ShareNet/TmpfsSize
- [x] buildBwrapArgs 命令构造完全可测（path.Join 保证 Linux 路径格式）
- [x] 测试：创建/无BaseDir错误/文件操作/bwrap参数构造/WorkingDir/ShareNet/UnshareNet/自定义只读/Close/非Linux执行

### 9.3 Daytona Workspace
- [x] `workspace/daytona.go`：DaytonaWorkspace（REST API 客户端）
- [x] 生命周期（POST /workspace 创建 → DELETE /workspace/{id} 销毁）
- [x] 文件操作（toolbox API：file/dir/mkdir/stat）
- [x] 命令执行（POST /toolbox/{id}/execute）
- [x] Bearer token 认证
- [x] 测试：创建+执行+关闭全生命周期/文件操作/缺配置/创建错误/认证头

### 9.4 OpenSandbox Workspace
- [x] `workspace/opensandbox.go`：OpenSandboxWorkspace（REST API 客户端）
- [x] 生命周期（POST /sandboxes 创建 → DELETE /sandboxes/{id} 销毁）
- [x] 文件操作（files/list/mkdir/stat API）
- [x] 命令执行（POST /execute）
- [x] 测试：完整生命周期（创建+执行+文件+列表+mkdir+stat+关闭）/缺配置

### 9.5 Apple Container Workspace
- [ ] `workspace/applecontainer.go`：macOS Containerization framework 适配（可选，build tag `darwin`）

### 9.6 示例与文档
- [ ] `examples/workspace_k8s/`：K8s workspace 端到端 demo + README
- [ ] `docs/WORKSPACE.md`：多后端 Workspace 使用指南

---

## Phase 10：存储深化 + 中间件精细化（P1，目标 2027-Q1）

### 10.1 SQL 存储后端
- [x] `service/sql_storage.go`：实现 `Storage` 接口，基于 `database/sql` + `modernc.org/sqlite`
- [x] 表结构声明（users/sessions/agents/credentials/messages/snapshots/schedules/teams）
- [x] SQLite 后端（`modernc.org/sqlite` 纯 Go，零 CGO）
- [x] 原子 upsert（`ON CONFLICT DO UPDATE`，支持自定义冲突列如 session_id）
- [x] 级联删除（删除 user → 级联删 sessions/messages/snapshots/agents/credentials/schedules/teams）
- [x] WAL 模式提升并发
- [x] 测试：User CRUD/upsert、Session/AgentConfig/Credential/Message/Snapshot/Schedule/Team CRUD、级联删除、接口断言

### 10.2 中间件钩子扩展
- [x] `middleware/middleware.go` 新增 `PermissionInterceptor`：`OnCheckPermission(ctx, agent, toolCall, tool, input, next) (PermissionResult, error)`（使用本地 `PermissionResult` 类型避免循环依赖）
- [x] `middleware/middleware.go` 新增 `CompressionInterceptor`：`OnCompressContext(ctx, agent, messages, next) error`
- [x] `middleware/chain.go` 更新：自动分类新钩子类型 + `ChainPermission` + `ChainCompression`
- [x] 向后兼容验证（不实现新接口则跳过）
- [x] 测试：权限拦截替换决策/绕过/nil chain / 压缩拦截/skip/error / Classify 新钩子

### 10.3 运行时状态注入
- [x] `middleware/injection.go`：`InjectionConfig`（timezone/time_format/time_interval/context_buffer_ratio/extra_fields/template）
- [x] `InjectionMiddleware`：SystemPromptTransformer + ReasoningInterceptor
  - [x] 自动注入当前时间（`<system-reminder>` 标签，间隔触发）
  - [x] extra_fields 用户自定义键值对
  - [x] 模板包裹 `{runtime_state}` 占位符
- [x] 测试：系统提示词/禁用/时间注入/间隔控制/extra_fields/模板/时区/Reasoning 追加消息

### 10.4 PowerShell 专用工具
- [x] `tool/shell/powershell.go`：`PowerShellTool`（自动探测 pwsh/powershell.exe）
- [x] Base64 UTF-16-LE 编码命令（`-EncodedCommand`）
- [x] `-NoLogo -NoProfile -NonInteractive` 标志
- [x] 输出上限 30,000 字符 + 超时上限 600s
- [x] 权限模型：所有命令强制 ASK（由 permission engine 控制）
- [x] 测试：编码/名称/Spec/空命令/截断/BaseDir/Timeout/IsReadOnly

### 10.5 资源跨用户共享
- [x] `service/access/policy.go`：`ResourceKind`(Credential/Agent/KB) + `ResourcePermission`(READ/EDIT) + `ResourceRef` + `Policy` 接口
- [x] `DenyAllPolicy`：默认拒绝跨用户访问（owner 总是能编辑自己的资源）
- [x] `StaticPolicy`：内存策略（测试/小部署用）
- [x] 测试：DenyAll list/canEdit owner/crossUser + Static list/canEdit read/edit/no-ref

---

## Phase 11：RAG 扩展 + 向量库补全（P2，目标 2027-Q1）

### 11.1 Word 解析器
- [x] `rag/parser/word.go`：`WordParser`，`.docx` 解析（纯 Go `archive/zip` + `encoding/xml`）
- [x] 段落提取（`<w:p>` + `<w:r>` + `<w:t>`）
- [x] 表格提取（`<w:tbl>`），Markdown pipe-table 渲染
- [x] 单测（端到端 .docx zip + 表格提取 + Markdown 渲染）

### 11.2 Excel 解析器
- [x] `rag/parser/excel.go`：`ExcelParser`，`.xlsx` 解析（纯 Go `archive/zip` + `encoding/xml`）
- [x] 逐 sheet 表格提取，Markdown pipe-table 渲染
- [x] SharedStrings 解析（`t="s"` 类型 cell → 索引解析）
- [x] 单测（数字 cell/共享字符串/多行/SharedStrings zip/Markdown 渲染）

### 11.3 Elasticsearch 向量库实现
- [ ] `memory/vector/elasticsearch_store.go`：替换占位，实现完整 `VectorStore` 接口
- [ ] 基于 `elastic/go-elasticsearch` 或 `olivere/elastic`
- [ ] `dense_vector` 索引映射 + cosine kNN 搜索
- [ ] SHA-256 确定性 ID + metadata filter
- [ ] 集成测试（`//go:build integration`）

### 11.4 MongoDB 向量库
- [ ] `memory/vector/mongodb_store.go`：新增 MongoDB 后端
- [ ] `$vectorSearch` 聚合 + Atlas/自托管支持
- [ ] 索引就绪轮询
- [ ] 集成测试

### 11.5 S3 BlobStore
- [ ] `rag/blob/s3.go`：基于 `aws/aws-sdk-go-v2`，实现 `BlobStore` 接口
- [ ] 集成测试

---

## v3 执行顺序建议

```
✅ 已完成  Phase 9.1-9.4  Workspace 沙箱扩展 (K8s/Bubblewrap/Daytona/OpenSandbox, 3→7 后端)
✅ 已完成  Phase 10.1    SQL 存储后端 (SQLite, 纯 Go, 零 CGO, 级联删除)
✅ 已完成  Phase 10.2    中间件 7 钩子扩展 (PermissionInterceptor + CompressionInterceptor)
✅ 已完成  Phase 10.3    运行时状态注入 (InjectionMiddleware: 时间/extra_fields)
✅ 已完成  Phase 10.4    PowerShell 专用工具 (pwsh/powershell.exe + EncodedCommand)
✅ 已完成  Phase 10.5    资源跨用户共享 (Policy/DenyAllPolicy/StaticPolicy)
✅ 已完成  Phase 11.1    Word 解析器 (.docx 段落+表格→Markdown)
✅ 已完成  Phase 11.2    Excel 解析器 (.xlsx sharedStrings+sheet→Markdown)

可选待做  Phase 9.5     Apple Container workspace (macOS only)
可选待做  Phase 11.3-5  ES/MongoDB 向量库 + S3 BlobStore
并行始终  守护任务       放大 A2A/ReMe/evolver/Plugin/ONNX
```

---

## Phase 12：Channel 多平台集成（P0，目标 2026-Q4）⚡本轮最高优先级

> 来源：`演进方案v4.md`（2026-08-08）。对标 Python `app/channel/`（Discord + 飞书/Feishu）。Go 完全空白，最大单点差距。

### 12.1 核心抽象 `channel/`
- [x] `channel/event.go` + `channel/channel.go`：`ChannelEvent`（ChannelID/UserID/ChatID/Text/MediaURLs/Metadata）+ `Channel` 接口（Start/SendText/Close）+ `Router` + `Runner`
- [x] `channel/gateway.go`：`Gateway`（HandleEvent：route → run，错误不杀 listener）+ `Registry` + `Dispatcher`（StartAll goroutine 生命周期）
- [x] `channel/routing.go`：`Binding`（exact>prefix>default）+ `RouteTable` + `ChatRouter`（channel→table 回退）
- [x] 核心单测 14 个（Gateway 路由/空事件/missing router / Registry CRUD / Dispatcher 启动 / Routing exact/prefix/default/no-match/fallback）

### 12.2 平台实现
- [x] `channel/webhook.go`：`WebhookChannel`（零依赖 HTTP 适配器：POST → ChannelEvent → emit，405/400/503 错误路径）——**完整闭环验证**（POST → normalize → route → run → reply）
- [x] webhook 单测（FullLoop + BadRequest 4 错误路径）
- [x] `channel/discord/`：**DiscordChannel**（`discordgo v0.29.0`：WebSocket Gateway + REST；handleMessageCreate 归一化→ChannelEvent + 自消息过滤 + 附件 MediaURLs；SendText REST 发送）——5 测试（归一化/自消息/空消息/接口断言/token 规范化/未连接错误/Close 幂等）
- [x] `examples/channel_discord/`：Discord bot demo（DISCORD_TOKEN 运行，chat→agent 路由 + echo 回复）
- [x] `channel/feishu/`：**FeishuChannel**（纯 HTTP 零依赖：tenant_access_token 2h 缓存自动刷新 + 事件订阅 webhook（challenge URL 验证 + im.message.receive_v1 归一化）+ REST 消息发送）——10 测试（token 缓存/发送/challenge/消息归一化/图片元数据/工具/错误路径）
- [x] 飞书 agent 工具：`feishu_send_message` + `feishu_list_chats`（SendMessageTool/ListChatsTool）
- [x] `examples/channel_feishu/`：飞书 bot demo + README（事件订阅 URL 配置说明）

### 12.3 Gateway 接入
- [x] `gateway/channel_runner.go`：`ChannelRunner`（AgentRegistry + SessionManager 适配，runAndReply 收集事件流文本 → SendText 回发，inflight 会话去重）
- [x] `gateway/channel_handlers.go`：`WithChannelGateway` + `StartChannels` + `RegisterChannelRoutes`（GET /api/v1/channels + POST /{id}/webhook）
- [x] `gateway/server.go`：channel 字段 + `Start()` 自动拉起 Dispatcher
- [x] `gateway/app.go`：RegisterAppRoutes 注册 channel 路由
- [x] **编译+测试验证通过**（临时以 v1.48.0 替换 v1.48.2 解锁 gateway 构建，验证后还原 go.mod；7 个集成测试：Runner 运行回复/inflight 去重/缺 agent/nil 降级 + Webhook 全链路/列表路由/404）

### 12.4 示例与测试
- [x] `examples/channel_webhook/`：零依赖 e2e demo（POST → 路由 dev-/qa-/default → echo 回复），**实测 200 + 闭环**
- [ ] `examples/channel_discord/`（需 discordgo 依赖）
- [x] gateway channel 集成测试（7 个，全绿）

---

## Phase 13：Hub 市场（P1，目标 2027-Q1）

> 对标 Python Hub 系统。从远程注册中心浏览+安装 MCP/Skill。

### 13.1 核心抽象 `hub/`
- [x] `hub/hub.go`：`Card`/`MCPCard`/`SkillCard` + `Hub` 接口（ListMCPCards/ListSkillCards + 游标分页）+ 泛型 `Page`/`FilterCards` 助手
- [x] `hub/install.go`：`InstallMCPs`（复用 `mcpserver.ConnectServers` 弹性连接，缺失二进制跳过）+ `InstallSkill`（HTTP 下载 + zip/tar/tar.gz 解压 + **zip-slip/tar-slip 防护** + 64MiB 下载/解压上限）
- [x] `hub/builtin/fs_hub.go`：`FSHub`（从 mcps.json + skills.json 加载目录，JSON 即市场）
- [x] 测试：分页边界/过滤/safeJoin 跨平台/zip+zip-slip/tar.gz+tar-slip/HTTP 下载/错误路径/MCP 弹性/FSHub 加载/分页过滤/坏 JSON（12+5 测试）

### 13.2 HTTP 路由 + 内置 Hub
- [x] `gateway/hub_handlers.go`：`WithHubs` + `RegisterHubRoutes`（GET /api/v1/hubs + /hubs/{id}/mcps|skills 浏览分页 + POST /hubs/{id}/mcps|skills/{card}/install 安装）
- [x] gateway 集成测试 6 个（列表/浏览/安装缺失二进制 422/skill 安装解压/404/无 hub 无路由）
- [x] `examples/hub_demo/`：可运行 demo（FSHub 浏览 + skill 下载解压安装），**实测通过**

---

## Phase 14：Console TUI + Workspace 服务化 + 治理-演化闭环（2026-08-21，目标 v2.6.0）

> 来源：`演进方案v5.md`。对标 Python v4 基准后 6 个实质提交（console #2297 / workspace 共享 #1951 / skills 隔离 #2283）+ v4 漏盘点的存量差距（artifact 端点 #2187 / git status #2257 / 中断原因 #2209）。

### 14.1 Console 终端 TUI（采用 bubbletea/bubbles/lipgloss）
- [x] `console/` 包：三态机（idle/running/confirming）+ `waitForEvent` tea.Cmd 桥接 ReplyStream + 三档 verbosity 渲染
- [x] HITL：逐工具 y/n/a 确认 → `InjectEvent(NewUserConfirmResult)` 恢复；Ctrl+C 拒绝全部
- [x] 中断：running 时 Ctrl+C → `agent.Interrupt()`（类型断言优雅降级）
- [x] `examples/console/`：calculator + ModeDefault 权限完整确认流
- [x] 8 测试全绿（文本流/截断/确认流/Ctrl+C 全拒/多工具逐个/陈旧事件/退出/truncate）

### 14.2 Workspace 服务化
- [x] artifact 端点：`GET /workspace/list_dir` + `read_file`（只读 + workspaceSafeJoin 防穿越 + 5MiB 上限）
- [x] status 端点：`GET /workspace/status`（工作目录 + git branch/porcelain，优雅降级）
- [x] 跨 agent 共享工作区：复用 `Session.WorkspaceID`（零 schema 改动）→ `<root>/<user>/shared/<name>`，路径段消毒
- [x] skills agent 级库 + `GET /workspace/agent_skills` + `POST /workspace/skill/select` 白名单选择（向后兼容）
- [x] 5 组集成测试全绿

### 14.3 治理-演化闭环 + steer/打断
- [x] auto-solidify：goal→completed 迁移触发 `evolver.Solidify`（DecisionSource=controlplane/PrimaryCause=goal_id），`AppConfig.AutoSolidifyOnGoalComplete` opt-in，异步+recover+防重复
- [x] steer/打断端点：`POST /v2/sessions/{id}/steer`（SessionManager.Steer 类型断言）+ `/{id}/interrupt`（复用 Terminate）
- [x] web_ui composer 注入/打断操作行
- [x] `examples/controlplane_demo` 第 10 步闭环演示（goal 完成 → capsule 固化 → 可列出）
- [x] 修复 `MockEvolver.Solidify` nil Gene panic；7 测试全绿

### 14.4 文档与遗留清理
- [x] AGENTS.md 更新（v2.5.1→目标 v2.6.0、controlplane/console 架构层、~71,100 行/165 包、设计决策 47-48）
- [x] `docs/WORKSPACE.md`：7 后端 + 会话工作区服务化指南
- [x] **多租户 SSE/WS 集成测试 + 安全修复**：发现 v2 POST/GET/DELETE/WS 不校验 session 归属 → `checkSessionAccess`（跨用户 404 不泄露存在性），2 组集成测试
- [x] react 中断恢复文案附带 partial response 摘要（#2209 对齐）+ 单测
- [x] 明确延后：Apple Container、ES/MongoDB 向量库、S3、NATS、K8s Operator（外部依赖，按需触发）

**Phase 14 验证**：`go build ./...` + `go test ./... -race -count=1` 全绿（93 包）；gofmt/vet 干净；新增 23 测试。

---

## Phase 15：对标 Python 36 新提交 + 护城河（2026-08-22，目标 v2.6.x）

> 来源：`演进方案v6.md`。基准 Python 801dd1ef（0d54503e 后 36 个非 merge 提交，主题"可观测性 + 工具精细化 + Team 增强"）。

### 15.1 KB 可观测性（对标 #2372）
- [x] `VectorStore.ListChunks` 接口 + InMemory 实现（doc_id+filter，chunk_index 排序，vector 剥离）
- [x] `GET /kb/{id}/documents/{doc}/chunks` + `/raw` 端点（worker 索引时注入 blob_uri 溯源 blob）
- [x] KB 列表富化 documents/chunks 计数；web_ui 分块懒加载 + 原文链接
- [x] 3 组测试全绿

### 15.2 工具增强（对标 #2114/#2378）
- [x] read_file 二进制 DataBlock（非 UTF-8 → base64 + media_type 嗅探 + 说明文本；文本行为不变）
- [x] `WithInputSchema(schema)` FunctionToolOption（覆盖自动 schema，typed handler 不受影响）
- [x] 核查：#2366（host OS 选 shell）在 Go 架构下不存在（shell 由各 Workspace 后端自决）

### 15.3 on_reply 续循环（对标 #2322）
- [x] `middleware.ErrContinueReply` 哨兵 + `Base.Call` 有界续循环（中间回复回灌 ReplyInput.Messages，3 轮上界）

### 15.4 Team 鲁棒性（对标 #2386，部分）
- [x] worker turn 失败 → leader inbox `<team-error>` + wakeup（300 字符截断，leader 自身不通知）
- [ ] max_image_num（#2362）/ loop 中间件（#2379）/ prompt cache tokens（#2318）等低价值密度项延后

### 护城河
- [x] `observability.ControlPlaneCollector`（9 个治理 counter）+ `Server.WithMetricsRegistry`（/metrics 自动接线）
- [x] `docs/benchmark_v2.6.0.md` 性能快照（147k req/s；1→100 并发延迟 +27%；热路径零/低分配）
- [ ] evolver 真实 MCP 后端 e2e（需外部服务）

**Phase 15 验证**：全仓库 `-race` 全绿；新增 14 测试（KB 3 + 工具 3 + 续循环 3 + team 1 + 指标 4）。

---

**仅剩 maintainer 执行**：`git tag v2.6.0 && git push --tags && gh release create v2.6.0 -F RELEASE_NOTES_v2.6.0.md`
