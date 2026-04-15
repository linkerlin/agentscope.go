# AgentScope.Go ReMe Memory 后续 TODO

> 本文件记录 ReMe Memory 系统在 agentscope.go 中的剩余工作项，按优先级排序。
> 最后更新：2026-04-15

---

## P1 - 高优先级

### 1. 多后端 VectorStore
- **现状**：仅实现了内存版 `LocalVectorStore`。
- **目标**：增加远程/持久化向量存储后端的 Go 客户端封装：
  - `ChromaVectorStore`
  - `QdrantVectorStore`
  - `ESVectorStore` (Elasticsearch)
  - `PGVectorStore` (PostgreSQL + pgvector)
- **影响文件**：新增 `memory/vector_store_chroma.go` 等。

### 3. ToolMemory 自动触发闭环
- **现状**：`ToolSummarizer` 已存在，但 `MemoryOrchestrator.Summarize` 中 Tool 分支为空（需要外部提供 `ToolCallResult`，无法从纯消息提取）。
- **目标**：
  - 在 Agent/Hook 层收集 `ToolCallResult` 并批量传递给 `ReMeVectorMemory`
  - 实现 `SummarizeToolUsage` 的自动调用与写入向量库
- **影响文件**：`memory/handler/orchestrator.go`、`tool` 包集成点。

---

## P2 - 中优先级

### 4. ReMeInMemoryMemory 独立抽象
- **现状**：`ReMeInMemoryMemory` 的能力（`marks`、`compSum`、`dialog` 持久化等）直接耦合在 `ReMeFileMemory` 内部。
- **目标**：提取独立结构体 `ReMeInMemoryMemory`，让 `ReMeFileMemory` 组合它而非内联所有字段。保持与 Python `ReMeInMemoryMemory` 的对齐。
- **影响文件**：`reme_file_memory.go`、新增 `reme_in_memory.go`。

### 5. 性能优化（并发、缓存）
- **目标**：
  - 为 `LocalVectorStore.Search` 引入读写锁优化或分片锁
  - 为 Embedding 调用增加本地缓存（LRU），避免重复文本的重复嵌入
  - 为异步摘要任务增加 goroutine 池或速率限制
- **影响文件**：`vector_store_local.go`、`embedding/`、`reme_file_memory.go`。

---

## P3 - 低优先级

### 6. 与 AgentScope-Java 功能对齐验证
- **目标**：对照 Java 版 AgentScope 的记忆接口与行为，验证 Go 版的一致性，编写跨语言对齐测试用例。

### 7. 配置系统深度整合
- **现状**：`config.ReMeMemoryConfig` 已定义，但尚未与 `ReMeVectorMemory` / `MemoryOrchestrator` 的构造函数打通。
- **目标**：实现 `config.BuildReMeVectorMemory(cfg config.ReMeMemoryConfig)` 工厂函数，一键从 YAML/JSON 配置初始化完整记忆系统。
- **影响文件**：`config/reme.go`。

### 8. 发布准备
- 完善 API 文档（godoc）
- 编写 CHANGELOG
- 发布前性能基准测试

---

*完成一项请勾选或删除对应条目，保持本文件实时更新。*
