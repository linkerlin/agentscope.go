# AgentScope.Go v2.4.0 Release Notes

> 🚀 **AgentScope.Go v2.4.0** —— 补齐"托管服务化"能力带：RAG 托管知识库、分布式消息总线协调原语、现代 Web UI、Agentic Memory，并守住 A2A/ReMe/evolver 独有护城河。
>
> 基于 2026-07-03 对 Python AgentScope main 分支的深度源码审阅，针对 Python 在"托管服务化"上完成的跃迁，用 Go 地道方式精准补齐。

---

## 核心亮点

### 1. RAG 托管知识库（对齐 Python `rag/` + `app/rag/` + `middleware/_rag.py`）

完整"文档→检索"托管管道，用 Go 地道方式补齐 Python 在此领域 90% 的差距：

- **7 子包全管道**：`rag/document`(Section/Chunk) → `rag/parser`(Text/PDF/PPTX/Image 4 解析器) → `rag/chunker`(ApproxTokenChunker) → `rag/blob`(LocalBlobStore) → `rag/kb`(KnowledgeBase + KBManager + InMemoryVectorStore) → `rag/index`(Worker + Queue)
- **4 解析器**：TextParser（纯 stdlib）、PDFParser（`ledongthuc/pdf` 纯 Go 无 CGO + panic recover）、PPTXParser（纯 stdlib `archive/zip`+`encoding/xml`）、ImageParser（内容嗅探 + 可选 OCR hook）
- **RAGMiddleware**：static/agent/both 三模式，OnReply 检索→OnReasoning 注入 HintBlock
- **KB HTTP API**：8 端点（CRUD + multipart/JSON 上传 + 搜索 + octet-stream 扩展名嗅探）
- **多租户隔离**：metadata_filter 强制写入每条记录

### 2. 分布式消息总线 CoordBus（对齐 Python `app/message_bus` 通用原语）

领域无关协调原语，多进程/分布式部署的统一骨干：

- **CoordBus 可选接口**：Lock / Registry / Queue / Log 四原语，不破坏现有 Bus/TeamBus
- **双后端**：LocalBus（channel 信号量锁 + slice+notify 队列 + map 注册表）/ RedisBus（SET NX PX + Lua token 释放 / HASH / RPUSH+BLPOP FIFO / LIST 游标）
- **跨会话投影**：SessionProjection 基于 CoordBus registry，无 CoordBus 优雅降级
- **CoordKeys 业务键约定** + WakeupKind（wake/resume）

### 3. 现代 Web UI 控制台（零构建，对齐 Python React Web UI）

- **零 npm/Node 构建依赖**：原生 vanilla JS + SSE + `go:embed`，单二进制部署
- **三视图**：Chat（AG-UI SSE 流式）/ Knowledge Bases（CRUD + 上传 + 检索）/ System（health + models）
- **GitHub 风深色主题**，前端直连 gateway HTTP API 无代理层

### 4. Agentic Memory 自主记忆（对齐 Python `AgenticMemoryMiddleware`）

agent **自己用已有文件工具**管理 Markdown 记忆文件（区别于 ReMe 被动检索 / LongTermMemory 工具模式）：

- `LocalMemoryStore`（frontmatter 解析）
- OnSystemPrompt 注入指令 + 有界 MEMORY.md 快照
- OnReply 异步检索 goroutine → OnReasoning 非阻塞注入 HintBlock 仅一次
- `FileSelector` 闭包解耦（默认 KeywordSelector，LLM 可 `WithSelector` 注入）

### 5. 工程化收尾

- **Tracing 语义属性提取**：Span 接口扩展 `SetAttributes`，五钩点提取（model/tool/iteration/usage/prompt.length），otelSpan 桥接真实 OTel
- **MCP 声明式配置**：ServerSpec YAML + 6-server 目录（filesystem/fetch/playwright/github/sqlite/brave-search）+ 弹性连接（未安装优雅跳过）
- **Langfuse 接入**：HTTP 批量 ingestion + Basic auth + trace/span/generation 事件映射
- **RBAC 测试**：补齐既有 RBAC 的 9 测试（权限矩阵 / 中间件 / 防提权 / 审计 CRUD / OrgID）
- **审计接线**：requireAuth 内置审计（自动记录 who/method/path/status，异步非阻塞）+ admin-only 查询路由
- **slog 结构化日志规范**：`logging/` 包（stdlib slog 封装 + LOG_LEVEL/LOG_FORMAT + FromContext 请求级）+ gateway RequestLoggingMiddleware

---

## 代码规模

| 指标 | v2.0.0 | v2.4.0 |
|------|--------|--------|
| 非测试代码行 | ~43,000 | **~66,500** |
| 测试代码行 | ~28,500 | **~39,900** |
| 测试文件 | ~250 | **303** |
| 包 | — | **162** |
| 示例 | 25 | **45** |

---

## 守住的护城河（Go 独有，继续领先）

- A2A 分布式协议（ShardRouter / ClusterManager / Redis 注册 / 安全 / 故障转移）
- ReMe 记忆（文件/向量/Hybrid/Dream/KG/SQLiteVec）
- GEP 自演化（evolver Gene/Capsule/Skill2GEP）
- Plugin 系统（.so 动态加载）
- ONNX 推理代理（CLIP/Whisper，零 CGO）
- 性能工程化（pprof + benchmark Catalog）

---

## 升级

```bash
go get github.com/linkerlin/agentscope.go@v2.4.0
```

无破坏性变更。新增能力均为可选接入（nil 降级）。

---

## 致谢

感谢 Python AgentScope 团队的参考实现。本轮演进基于对其 main 分支的深度审阅，用 Go 地道方式补齐"托管服务化"能力带。
