# AgentScope.Go 演进实施 TODO

> 来源：`演进方案.md`（2026-07-03 深度修订版，v2）
> 状态：**Phase 5-8 + 旧 Phase 3 收尾全部实质完成**（v2.4.0）
> 更新日期：2026-07-04（全量代码审阅后）
>
> **代码规模**（2026-07-04 审阅实测）：~66,500 非测试行 / ~39,900 测试行 / 303 测试文件 / 162 包 / 45 示例 / `go build ./...` + `go vet ./...` 全绿
>
> **本轮（Phase 5-8 + 旧3）已完成**：
> - **Phase 5 RAG 托管知识库**：rag/{document,parser(4),chunker,blob,kb,index} + RAGMiddleware + KB HTTP API + examples/rag_kb
> - **Phase 6 消息总线深化**：CoordBus 四原语(Lock/Registry/Queue/Log) Local+Redis + 跨会话投影
> - **Phase 7 现代 Web UI**：零构建 SPA 控制台(Chat/KB/System) + go:embed
> - **Phase 8 Agentic Memory + Tracing**：文件式自主记忆 + Span 语义属性提取 + otelSpan 桥接
> - **旧 Phase 3 收尾**：MCP 声明式配置(catalog+YAML) + Langfuse 接入 + RBAC 测试 + 审计接线(requireAuth 内置) + slog 规范(logging 包 + RequestLoggingMiddleware)
>
> **仅剩 maintainer 执行**：`git tag v2.4.0 && git push --tags && gh release create v2.4.0 -F RELEASE_NOTES`

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

- [ ] A2A：打造"多语言 Agent 网格"事实标准（文档 + 跨语言示例）
- [ ] evolver：闭环落地"自愈 Agent"卖点（真实 MCP 后端）
- [ ] ONNX：发布本地多模态零 CGO benchmark 报告
- [ ] Plugin：生态插件市场雏形（README + 示例插件）

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

## 本轮执行顺序建议

```
P0 立即   Phase 5.1-5.2  document + parser + chunker（纯 Go 库，无外部依赖，可独立验证）
          Phase 5.3-5.5  blob + KBManager + IndexWorker（管道打通）
          Phase 5.6-5.7  RAGMiddleware + KB Router（服务暴露）
P1 次优   Phase 6        消息总线 + 服务编排（Phase 5 IndexWorker 依赖）
P2        Phase 7        现代 Web UI
P3        Phase 8        记忆/可观测/存储/工具
并行始终  守护任务       放大 A2A/ReMe/evolver/Plugin/ONNX
```
