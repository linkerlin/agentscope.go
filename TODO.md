# AgentScope.Go 演进实施 TODO

> 来源：`演进方案.md`  
> 状态：Phase 1 核心任务已完成；Phase 2 / Phase 3 已完成  
> 更新日期：2026-06-13
>
> **本次会话完成**：
> - 社区治理文件、GitHub 模板
> - CHANGELOG / MIGRATION / RELEASE_CHECKLIST
> - 部署文档扩展、Studio / A2A / Evolver / API 参考文档
> - 5 分钟快速上手教程 `docs/quickstart.md`
> - Phase 2 视频脚本大纲（中英文）`docs/video-script-phase2.md`
> - CI 增强（跨平台、覆盖率、govulncheck、stale、pr-title-check）
> - 模型示例脚本库（13 个示例：10 provider + multiagent / stream / multimodal）
> - Cookbook（5 个 recipes：MapReduce、审稿、RAG、定时报告、自愈 Agent）
> - Studio 升级：模型卡片页、Schedule 自动刷新列表、ReMe 检索调试面板、A2A Registry 浏览器、Evolver gene/capsule 面板、修复 API 路由委托
> - README / tutorial 更新
> - `version.go` 升级到 `v2.0.0`，同步 AGENTS.md / issue 模板 / deployment.md
> - `memory/` 全局 `gofmt` 格式化
> - 修复 `async/pool.go` 中任务状态与结果更新的竞态条件
> - Phase 3 向量数据库连接器：Milvus / Qdrant / Chroma 完整 REST API 实现
> - `memory/vector/docker-compose.yml` + `integration_test.go` + `README.md`
> - DashScope 多模态嵌入：`embedding.NewDashScopeMultimodal`
> - Rerank 抽象与 3 个后端实现：`rerank/` 包（Noop / Cohere / Jina / Local cosine）
> - Rerank 接入 `ReMeVectorMemory`：`WithReranker()` + `RetrieveMemory` 二阶段精排
> - RAG + Rerank Cookbook：`cookbook/rag_with_rerank/`
> - `rerank/` 单元测试：覆盖 Noop、Local、Cohere（mock）、Jina（mock）
> - Phase 4 A2A 分布式注册中心：`a2a/store.go` + `a2a/store_redis.go` + `a2a/router.go`
> - 一致哈希分片路由：`a2a.ShardRouter`
> - 示例 `examples/a2a_redis_registry/`（支持真实 Redis 或嵌入式 miniredis）
> - Agent 故障转移 watch 机制：`a2a/watch.go`、`Registry.Watch(ctx)`、`ShardRouter.AutoRefresh`
> - 全量构建 / 测试 / `gofmt` 验证全部通过
>
> **剩余需 maintainer 执行**：打 tag `v2.0.0`、创建 GitHub Release。

---

## 实施原则

1. **每批次改动后必须执行**：`go build ./...` + `go test ./... -race -count=1 -timeout=12m`
2. **优先落地 Phase 1**：发布准备、社区模板、文档、CI、示例脚本、测试补齐
3. **保持向后兼容**：已有 public API 不破坏；新增功能用新文件/新包
4. **示例即文档**：每个新增示例必须可独立运行，并配有 README 说明

---

## Phase 1：发布与文档（2026-Q3 目标）

### 1.1 发布准备
- [x] 创建/完善 `CHANGELOG.md`，遵循 Keep a Changelog 格式
- [x] 创建 `MIGRATION.md`：从 v1/v2-rc 迁移到 v2.0.0 的指南
- [x] 创建 `RELEASE_CHECKLIST.md`：发布流程清单
- [x] 更新 `version.go` 为 `v2.0.0`
- [ ] 创建 Git tag `v2.0.0` 与 GitHub Release（需 maintainer 执行）

### 1.2 社区与治理
- [x] 创建 `CONTRIBUTING.md`：贡献流程、PR 规范、commit message 规范
- [x] 创建 `CODE_OF_CONDUCT.md`
- [x] 创建 `SECURITY.md`：安全策略、CVE 报告流程
- [x] 创建 `.github/ISSUE_TEMPLATE/bug_report.md`
- [x] 创建 `.github/ISSUE_TEMPLATE/feature_request.md`
- [x] 创建 `.github/ISSUE_TEMPLATE/rfc.md`
- [x] 创建 `.github/pull_request_template.md`
- [x] 更新 `README.md`：添加贡献者指引、社区入口

### 1.3 CI/CD 强化
- [x] 更新 `.github/workflows/ci.yml`：增加 Windows、macOS 运行器
- [x] 新增 `.github/workflows/coverage.yml`：生成覆盖率报告并上传 codecov
- [x] 新增 `.github/workflows/govulncheck.yml`：依赖安全扫描
- [x] 新增 `.github/workflows/stale.yml`：stale issue/PR 管理
- [x] 新增 `.github/workflows/pr-title-check.yml`：Conventional Commits 标题校验
- [x] 确认 `Makefile` 已包含 `cover`、`bench` 目标

### 1.4 文档补齐
- [x] 创建 `docs/MIGRATION.md`（从根目录同步）
- [x] 扩展 `docs/DEPLOYMENT.md`：Docker、systemd、K8s、Redis 部署
- [x] 扩展 `docs/tutorial.md`：增加每家模型后端的快速上手
- [x] 扩展 `docs/api-reference.md`：补充 gateway、service、a2a、evolver、observability 端点
- [x] 创建 `docs/STUDIO.md`：Studio 使用与自定义指南
- [x] 创建 `docs/A2A.md`：A2A 协议使用指南
- [x] 创建 `docs/EVOLVER.md`：GEP 自演化教程

### 1.5 测试补齐
- [x] 为 `tool/schedule/` 4 个工具编写单元测试（已存在并验证通过）
- [ ] 为 `gateway/` 增加多租户 SSE/WS 集成测试
- [ ] 提升 `memory/`、`gateway/` 外其他模块的覆盖率
- [ ] 标记 integration tests 为 `//go:build integration`

### 1.6 模型示例脚本库
- [x] 创建 `scripts/model_examples/` 目录
- [x] 实现 OpenAI Chat call 脚本
- [x] 实现 OpenAI Chat multiagent 脚本
- [x] 实现 OpenAI Chat stream 脚本
- [x] 实现 OpenAI Chat multimodal 脚本
- [x] 实现 OpenAI Response API call 脚本
- [x] 实现 Anthropic call 脚本
- [x] 实现 Gemini call 脚本
- [x] 实现 DashScope call 脚本
- [x] 实现 DeepSeek call 脚本
- [x] 实现 Moonshot call 脚本
- [x] 实现 xAI call 脚本
- [x] 实现 Ollama call 脚本
- [x] 实现 vLLM call 脚本
- [x] 为脚本库编写 README

---

## Phase 2：Studio 与示例（2026-Q4 目标）

### 2.1 Studio 升级
- [ ] 评估 Templ 替代 html/template 的可行性（Phase 4 可选）
- [ ] 为 Studio 引入 Tailwind CSS CDN/内嵌（Phase 4 可选）
- [x] Studio 增加模型卡片列表页
- [x] Studio 增加 Schedule 可视化列表（含自动刷新与状态）
- [x] Studio 增加 ReMe 检索调试面板
- [x] Studio 增加 A2A Registry 浏览器
- [x] Studio 增加 evolver gene/capsule 管理面板
- [x] 修复 Studio 中 Gateway API 路由未委托到 Server 的问题
- [ ] 验证 Python Studio UI 可连接 Go Gateway（Phase 3/4）

### 2.2 Cookbook
- [x] 创建 `cookbook/README.md`
- [x] 长文档摘要（MapReduce）
- [x] 多 Agent 审稿流程
- [x] RAG 问答
- [x] 定时报告 Agent
- [x] 自愈 Agent（evolver + gateway MCP）

### 2.3 教程与视频
- [x] 编写 5 分钟快速上手 Markdown 教程
- [x] 准备 Phase 2 视频脚本大纲（中文 + 英文）
- [ ] 录制视频（Phase 4 可选）

---

## Phase 3：生态集成（2027-Q1 目标）

### 3.1 RAG 与向量数据库
- [x] 实现 Milvus 连接器（REST API v2）
- [x] 实现 Qdrant 连接器（REST API）
- [x] 实现 Chroma 连接器（REST API v1，已有，改为懒加载）
- [x] 补齐 DashScope 多模态嵌入 `embedding.NewDashScopeMultimodal`
- [x] `memory/vector/docker-compose.yml` + `integration_test.go` + `README.md`
- [x] RAG 管道：Rerank 集成
  - [x] 抽象 `rerank.Reranker` 接口与 `NoopReranker`
  - [x] Cohere Rerank v2 后端
  - [x] Jina Rerank 后端
  - [x] 本地余弦相似度 Rerank 后端
  - [x] 接入 `ReMeVectorMemory.WithReranker()`
  - [x] RAG + Rerank Cookbook
  - [x] `rerank/` 单元测试

### 3.2 MCP 生态
- [ ] 维护 `agentscope-go-mcp-servers` 列表
- [ ] 提供 filesystem/web-search/browser/github MCP 配置示例

### 3.3 企业级功能
- [ ] OIDC/OAuth2 SSO 支持
- [ ] RBAC 角色权限
- [ ] 组织/工作空间隔离
- [ ] 审计日志

### 3.4 可观测性
- [ ] Prometheus metrics 端点（Gateway 已暴露 `/metrics`，需补充业务指标）
- [ ] structured logging（slog）规范
- [ ] Langfuse 接入示例

---

## Phase 4：前沿能力（2027-Q2 目标）

### 4.1 实时语音 Agent
- [ ] 基于 pion/webrtc 的语音对话示例
- [ ] STT → LLM → TTS 全管道流式
- [ ] 打断与 VAD 支持

### 4.2 Agentic RL / 微调
- [ ] Tinker/LLaMA-Factory/Unsloth 接口预留
- [ ] 从运行日志生成 SFT/RL 训练数据
- [ ] 在线学习闭环示例

### 4.3 分布式 Agent
- [x] A2A + Redis 分片与负载均衡（初版）
  - [x] `RegistryStore` 抽象，`Registry` 可插拔后端
  - [x] `RedisRegistryStore`：基于 Redis 的注册中心持久化
  - [x] `ShardRouter`：基于一致哈希的健康节点路由
  - [x] `examples/a2a_redis_registry` 可运行示例
- [x] Agent 故障转移 watch/通知机制
  - [x] `Registry.Watch(ctx)` 本地变化事件流
  - [x] `ShardRouter.AutoRefresh(ctx, interval)`：Watch 即时刷新 + 轮询兜底
  - [x] `a2a/watch_test.go` 覆盖注册/移除/健康变化/自动刷新
- [ ] NATS 后端支持（可选）
- [ ] K8s Operator（可选）

### 4.4 自演化闭环
- [ ] evolver 真实 MCP 后端集成
- [ ] Studio 展示 gene/capsule 生命周期
- [ ] 生产级自愈 Agent 示例

---

## 当前会话实施计划

本次会话完成 Phase 4 中“A2A 分布式注册中心与分片路由”及“Agent 故障转移 watch 机制”核心任务：

1. 引入 `a2a.RegistryStore` 抽象，重构 `Registry` 支持可插拔存储后端
2. 实现 `RedisRegistryStore`（基于 `go-redis`，含 `miniredis` 单元测试）
3. 实现 `a2a.ShardRouter` 一致哈希分片路由
4. 创建 `examples/a2a_redis_registry` 可运行示例（支持真实 Redis 或嵌入式 miniredis）
5. 新增 `Registry.Watch(ctx)` 本地变化事件流与 `ShardRouter.AutoRefresh` 自动刷新
6. 全量 `go build ./...` + `go test ./... -race -count=1 -timeout=12m` 验证

下一步（Phase 4 剩余）：NATS 后端支持、MCP 配置示例、OIDC/RBAC、Prometheus 业务指标。
