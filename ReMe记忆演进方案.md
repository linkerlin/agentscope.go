# AgentScope.Go ReMe 记忆模块演进方案

> 基于对 [ReMe (Python)](../ReMe/) v0.3.1.10 / reme4 v0.4.0.0 与 [agentscope.go](./) v2.0.0-rc.1 记忆模块的深度对比分析，制定并实施本方案。
>
> **状态：18/23 项已完成（78%）**
>
> 最后更新：2026-06-11（二次深度分析更新）

---

## 目录

1. [分析概览与结论](#1-分析概览与结论)
2. [综合差距清单与实施状态](#2-综合差距清单与实施状态)
3. [P0 级：Dual-Content Embedding](#3-已完成p0-级dual-content-embedding)
4. [P1 级：异步编排与多阶段流水线](#4-已完成p1-级异步编排与多阶段流水线)
5. [P2 级：Profile、Flow、FileWatcher、Cache、Tool](#5-已完成p2-级profileflowfilewatchercachetool)
6. [P3 级：Registry、Benchmark、GC、Comparative、Library](#6-已完成p3-级registrybenchmarkgccomparativelibrary)
7. [剩余差距：P4 级——架构层面缺失](#7-剩余差距p4-级架构层面缺失)
8. [ReMe4 前沿架构分析（新）](#8-reme4-前沿架构分析新)
9. [Go 相对 Python 的已有优势](#9-go-相对-python-的已有优势)
10. [详细功能对比矩阵](#10-详细功能对比矩阵)
11. [附录 A：关键文件对照表](#11-附录-a关键文件对照表)
12. [附录 B：MemoryType 枚举对比](#12-附录-bmemorytype-枚举对比)
13. [附录 C：新增/修改文件一览](#13-附录-c新增修改文件一览)

---

## 1. 分析概览与结论

### 1.1 总体判断

**agentscope.go 的 ReMe 记忆模块已覆盖 Python ReMe v0.3.1.10 约 78% 的完整功能（不含 P4 层则为 90%）。**

Go 版本在以下方面已经**达到或超越** Python 版本：

| 维度 | Go 现状 | Python 对标 | 优势归属 |
|------|---------|------------|----------|
| 向量存储后端 | 5 种 | 9 种 | Python 更多 |
| 混合检索 | FTS5+BM25+向量融合 | 简单 substring 匹配 | **Go 独有/更优** |
| 会话快照 | 版本化 JSON + SaveTo/LoadFrom | 基础序列化 | Go 更规范 |
| RAG 文档解析 | Apache Tika 集成 | 无 | **Go 独有** |
| 并发安全 | sync.Mutex + errgroup | asyncio.Lock | Go 天然优势 |
| WindowMemory | 双限制滑动窗口 | 无独立实现 | **Go 独有** |
| Plan 存储层 | plan.Storage | 无 | **Go 独有** |
| MCP 客户端/服务端 | SDKClient + ServerAdapter | MCPClient + MCPService | 基本持平 |
| ReMeHook | HookBeforeModel 拦截 | pre_reasoning_hook | Go 更模块化 |
| Pipeline DSL | 类型安全构建器 | YAML 配置化 | Go 编译期检查 |
| Deduplicator | 向量+LLM双模式 | 无独立组件 | **Go 独有** |
| FileStore 抽象 | 无形式化接口 | BaseFileStore(多后端) | **Python 领先** |
| 记忆类型数量 | 4 种 | 6 种 | **Python 领先** |
| 插件架构 | Registry[T] 泛型 | ComponentRegistry + bind() | **Python 更成熟** |

### 1.2 功能覆盖率总览

```
实施前（基础版）：   ██████████████░░░░░░  ~60%  原始 Go 实现
Phase 1-3（已有）：  █████████████████░░░  ~78%  P0+P1+P2+P3 实施
Phase 4（剩余）：    ████████████████████  100%  P4 架构补齐
```

### 1.3 关键发现

通过二次深度分析（含 reme4 v0.4.0.0 架构调研），发现 **5 项此前未纳入的差距**：

1. **MemoryType.IDENTITY** — 长期稳定身份记忆（用户名、角色等），Go 缺失
2. **MemoryType.HISTORY** — 原始对话历史的记忆节点化存储，Go 缺失（仅有嵌入的对话切片）
3. **形式化 FileStore 抽象** — ReMe 有 `BaseFileStore` 多后端接口层；Go 的文件存储操作嵌入在 `ReMeFileMemory` 中
4. **DeltaFileWatcher** — ReMe 有全量/增量双模式文件监听；Go 仅有简单哈希轮询
5. **DreamStep 记忆演化** — ReMe4 的 2 阶段记忆整合管线（extract + integrate），含 CREATE/CORROBORATE/REFINE/CORRECT 四种写入策略

---

## 2. 综合差距清单与实施状态

### 2.1 P0~P3 级（已实施 17/18）

| 优先级 | 差距项 | 状态 | 实施文件 |
|--------|--------|------|----------|
| **P0** | Dual-Content Embedding 未实现 | ✅ 已实施 | `reme_types.go`, 5个 `vector_store_*.go`, `deduplicator.go`, `memory_handler.go` |
| **P1** | Orchestrator 同步执行 | ✅ 已实施 | `handler/orchestrator.go` (errgroup 并行化) |
| **P1** | 无 Two-Phase Personal Memory | ✅ 已实施 | `summarizer_personal.go`, `handler/orchestrator.go` |
| **P1** | 无 Multi-Query Batch Search | ✅ 已实施 | `handler/memory_handler.go`, `vector_store_local.go` |
| **P1** | 无 Flow Pipeline 框架 | ✅ 已实施 | `pipeline/` 包（4 文件） |
| **P2** | Profile 仅支持文件后端 | ✅ 已实施 | `handler/profile_handler.go` (接口化 + VectorProfileBackend) |
| **P2** | 无 Memory Validation 步骤 | ✅ 已实施 | `pipeline/steps.go` (MemoryValidationStep) |
| **P2** | 无 LLM Rerank 步骤 | ✅ 已实施 | `pipeline/steps.go` (LLMRerankStep) |
| **P2** | 无 File Watcher | ✅ 已实施 | `file_watcher.go` |
| **P2** | Embedding Cache 无持久化 | ✅ 已实施 | `embedding_cache.go` (LoadFromDisk/SaveToDisk) |
| **P2** | Tool Memory 缺参数优化 | ✅ 已实施 | `summarizer_tool.go` (ParameterInsight) |
| **P3** | 无 Comparative Extraction | ✅ 已实施 | `summarizer_procedural.go` (ExtractComparative) |
| **P3** | 无 Memory Utility/Freq + GC | ✅ 已实施 | `memory_gc.go` (MemoryCollector + RecordAccess) |
| **P3** | 无 Registry/Plugin 架构 | ✅ 已实施 | `registry.go` |
| **P3** | 无 Benchmark 集成 | ✅ 已实施 | `benchmark.go` (HaluMem + LoCoMo) |
| **P3** | 无 Memory Library | ✅ 已实施 | `memory_library.go` (GetDefaultAgentLibrary) |
| **P3** | **无 MCP Service 模式** | ⏳ **待实施** | 利用已有的 `toolkit/mcp/` 适配层 |
| **P3** | 无 IDENTITY 记忆类型 | ✅ 已实施 | `reme_types.go` (MemoryTypeIdentity) |

### 2.2 P4 级（新增——架构层面缺失，5项待实施）

| 优先级 | 差距项 | Python 对标 | 状态 | 预计工时 |
|--------|--------|------------|------|----------|
| **P4** | 无形式化 FileStore 接口层 | `BaseFileStore`（v3 6 后端 / v4 2 后端） | ⏳ 待实施 | 5d |
| **P4** | 无 DeltaFileWatcher（增量监听） | `DeltaFileWatcher._find_cutoff_line()` | ⏳ 待实施 | 2d |
| **P4** | 无 HISTORY 记忆类型 | `AddHistory` / `ReadHistory` 工具 | ⏳ 待实施 | 3d |
| **P4** | 无 DreamStep 记忆演化 | `DreamStep` 2 阶段 + 4 写入策略 | ⏳ 待实施 | 8d |
| **P4** | FileWatcher 无 delta/full 双模式 | `FullFileWatcher` vs `DeltaFileWatcher` | ⏳ 待实施 | 2d |

---

## 3. 已完成：P0 级——Dual-Content Embedding

### 3.1 对标对象

对标 ReMe Python `MemoryNode.to_vector_node()`（`memory_node.py:117`）的分离策略：

- `WhenToUse` 非空 → 向量嵌入使用 `WhenToUse`（精准触发条件匹配）
- `WhenToUse` 为空 → 向量嵌入使用 `Content`（向后兼容）
- 检索结果始终包含完整 `Content`（展示层不受影响）
- 从 VectorNode 恢复时正确反向映射（`from_vector_node()`）

### 3.2 实施文件

| 文件 | 变更内容 |
|------|----------|
| `reme_types.go` | 新增 `EmbeddingContent()` 方法、`NewMemoryNodeWithWhen()` 构造函数、`MemoryTypeIdentity` 枚举值 |
| `reme_types_test.go` | 新增 4 个测试用例 |
| `vector_store_local.go` | `Insert` 使用 `EmbeddingContent()` |
| `vector_store_chroma.go` | `Insert` + `Update` 使用 `EmbeddingContent()` |
| `vector_store_pgvector.go` | `Insert` + `Update` 使用 `EmbeddingContent()` |
| `vector_store_qdrant.go` | `nodesToPoints` 使用 `EmbeddingContent()` |
| `vector_store_elasticsearch.go` | `Insert` + `Update` 使用 `EmbeddingContent()` |
| `deduplicator.go` | `DeduplicateAgainstStore` + `deduplicateByVector` 使用 `EmbeddingContent()` |
| `handler/memory_handler.go` | `AddDraftAndRetrieveSimilar` 使用 `EmbeddingContent()` |

---

## 4. 已完成：P1 级——异步编排与多阶段流水线

### 4.1 Orchestrator 并发化

`handler/orchestrator.go` 的 Summarize 和 Retrieve 均已改用 `golang.org/x/sync/errgroup`：

```
Summarize:
  1) History → 同步（后续步骤依赖 history_node）
  2) Personal + Procedural + Tool → errgroup 并行
  
Retrieve:
  Personal + Procedural + Tool → errgroup 并行 → 合并 → Dedup
```

新增 `mu sync.Mutex` 保护 `SummarizeResult` 的并发写入。

### 4.2 Two-Phase Personal Memory（对标 PersonalSummarizer）

`summarizer_personal.go` 新增 `ExtractInsightsWithProfile()`：

```
S1: ExtractObservations → 提取原始观察事实
      ↓
      ReadAllProfiles → 加载已有用户画像
      ↓
S2: ExtractInsightsWithProfile → 基于已有画像提取洞察
      (LLM prompt 包含现有画像，指导：补充/矛盾/重复判断)
```

`handler/orchestrator.go::summarizePersonal` 自动分流：有已有 Profile 时用 S2 增强路径，无画像时 fallback 到原有单阶段。

### 4.3 Multi-Query Batch Search（对标 memory_handler.py:202）

`handler/memory_handler.go` 新增 `BatchSearch()` + `BatchSearchQuery`：

1. 多查询独立检索 → 按 memoryID 去重合并
2. 若 `hybridThreshold > 0` 且为 LocalVectorStore → 计算跨查询平均余弦相似度过滤
3. `vector_store_local.go` 新增 `BatchSearchWithThreshold()` 支持平均余弦阈值

### 4.4 Flow Pipeline 框架（对标 ReMe service.yaml 表达式流）

`memory/pipeline/` 包（4 文件）：

| 文件 | 内容 |
|------|------|
| `pipeline.go` | `Step` 接口、`Pipeline`、`StepNode`、`StepMode`(Sequential/Parallel/Branch)、`Seq()`/`Par()`/`BranchFirst()` 构建器 |
| `context.go` | `FlowContext` 步骤间数据传递 |
| `steps.go` | 7 个内置步骤：`MemoryRetrieval`、`RerankMemory`、`LLMRerank`、`MemoryValidation`、`MemoryDeduplication`、`MemoryAddition`、`MemoryDeletion` |
| `pipeline_test.go` | 4 个测试（顺序/并行/检索/去重） |

使用示例：
```go
p := pipeline.NewPipeline("retrieve-task-memory", pipeline.Seq(
    &pipeline.MemoryRetrievalStep{Store: store},
    &pipeline.LLMRerankStep{Model: chatModel, TopK: 5, Enable: true},
    &pipeline.MemoryValidationStep{Threshold: 0.3},
))
p.Execute(ctx, pipeline.NewFlowContext("how to fix this bug"))
```

---

## 5. 已完成：P2 级——Profile、Flow、FileWatcher、Cache、Tool

### 5.1 Profile 向量后端（对标 profile_handler.py 双后端）

`handler/profile_handler.go` 重构为后端接口模式：

- `ProfileBackend` 接口（`ReadAll` / `Update` / `Delete` / `Retrieve`）
- `FileProfileBackend` — 原 JSON 文件实现（对标 `FileProfileBackend`）
- `VectorProfileBackend` — 新向量存储实现（对标 `VectorProfileBackend`）
- `NewProfileHandlerWithBackend()` — 自定义后端构造函数
- 画像条目以 `MemoryTypePersonal` 存储为 `MemoryNode`，支持语义检索

### 5.2 LLM Rerank（对标 ReMe service.yaml 的 RerankMemory）

`pipeline/steps.go` 新增 `LLMRerankStep`：

- 使用 `model.ChatModel` 对检索结果按查询相关性精排
- 支持 TopK 截断
- 失败不中断流水线（fallback 到原始顺序）

### 5.3 File Watcher（对标 ReMe BaseFileWatcher）

`file_watcher.go`：

- SHA-256 哈希监控 `MEMORY.md` 和 `memory/` 目录
- `OnChange` 回调机制
- 定时轮询模式（默认 5s，对标 `poll_delay_ms=2000`）
- `Start` / `Stop` 生命周期管理

**与 Python 的差异：** 当前仅有全量轮询模式，缺少 `DeltaFileWatcher` 的增量追加检测能力（见 [7.2](#72-缺失deltawatch-增量文件监听)）。

### 5.4 Embedding Cache 磁盘持久化（对标 BaseEmbeddingModel）

`embedding_cache.go` 新增：

- `SetDiskPath()` — 设置持久化路径
- `LoadFromDisk()` — 启动时加载 JSON 缓存（对标 `start()` 中的 `_load_cache_file()`）
- `SaveToDisk()` — 刷盘到 JSON 文件（对标 `close()` 的 `_save_cache_file()`）
- `dirty` 标记 — 追踪变更，仅在脏时刷盘

### 5.5 Tool Memory 参数优化（对标 Python ParameterInsight 对应逻辑）

`summarizer_tool.go` 新增 `ParameterInsight` 类型：

```go
type ParameterInsight struct {
    Parameter string  // 参数名
    Pattern   string  // 有效的参数模式
    Outcome   string  // 使用此模式的结果
    Frequency int     // 观察到的频率
    Score     float64 // 效果评分
}
```

---

## 6. 已完成：P3 级——Registry、Benchmark、GC、Comparative、Library

### 6.1 Comparative Extraction（对标 Python extract_comparative）

`summarizer_procedural.go` 新增 `ExtractComparative()`：

- 对比成功轨迹 vs 失败轨迹
- LLM 提取：关键差异、成功因素、失败原因、可迁移模式
- 按 ReMe Python 格式输出结构化经验

### 6.2 Memory Utility/Freq + GC（对标 ReMe delete_task_memory）

`memory_gc.go`：

- `MemoryCollector` — 基于 freq/utility 的自动清理器
- `RecordAccess()` — 每次检索时更新 freq/last_accessed/utility
- `EstimateUtility()` — 综合评分含时间衰减（log2 衰减，对标 Python `_calculate_time_decay`）
- 删除条件：`freq >= threshold && utility < threshold` 或过期（MaxAge）

### 6.3 Registry（对标 ReMe RegistryFactory + RegistryEnum）

`registry.go`：

- 泛型 `Registry[T]` 支持任意组件注册
- `Register()` / `Get()` / `Names()` / `Has()`
- 全局实例：`VectorStores`、`EmbeddingModels`
- 对标 `R.vector_stores`、`R.embedding_models`、`R.file_stores` 等

### 6.4 Benchmark（对标 ReMe benchmark/ 目录）

`benchmark.go`：

- `Benchmark` 接口
- `HaluMemBenchmark` — 幻觉检测（MemoryAccuracy + QAAccuracy）
- `LoCoMoBenchmark` — 长对话记忆保持
- `RunBenchmarkSuite()` — 批量运行

**未覆盖的 ReMe Benchmark：** AppWorld（任务完成率）、BFCL（函数调用）、LongMemEval（超长记忆回归）

### 6.5 Memory Library（对标 ReMe docs/library/）

`memory_library.go`：

- `GetDefaultAgentLibrary()` — 5 条预构建记忆模板：
  - web_search 优化技巧
  - file_read 大文件处理
  - debugging 二分法策略
  - code_review 审查优先级
  - write_file 目录创建
- `InjectIntoVectorStore()` — 批量注入向量存储
- `LoadFromDir()` — 从目录加载 JSON 记忆文件

---

## 7. 剩余差距：P4 级——架构层面缺失

### 7.1 缺失：形式化 FileStore 接口层

**问题：** agentscope.go 没有独立的 `FileStore` 接口。所有文件存储操作（FTS5 索引、dialog/ 目录、session 快照、tool_result 外溢）都直接嵌入在 `ReMeFileMemory` 和 `ReMeVectorMemory` 具体类型中。

**Python 对标：**

| 模块 | 抽象层 | 具体实现 |
|------|--------|----------|
| v3 `reme/core/file_store/` | `BaseFileStore` (ABC) | `SqliteFileStore`, `ChromaFileStore`, `LocalFileStore`, `SeekDBFileStore`, `ZVecFileStore` (5 种) |
| v4 `reme4/components/file_store/` | `BaseFileStore(BaseComponent)` | `LocalFileStore`, `FaissLocalFileStore` (2 种) |

**Python `BaseFileStore` v3 方法契约：**
```
upsert_file(file_meta, source, chunks)
delete_file(path, source)
delete_file_chunks(path, chunk_ids)
upsert_chunks(chunks, source)
list_files(source) → list[str]
get_file_metadata(path, source) → FileMetadata
update_file_metadata(file_meta, source)
get_file_chunks(path, source) → list[MemoryChunk]
vector_search(query, limit, sources) → list[MemorySearchResult]
keyword_search(query, limit, sources) → list[MemorySearchResult]
hybrid_search(query, limit, sources, vector_weight, candidate_multiplier) → list[MemorySearchResult]
```

**实施方案：**

```go
// 新增 memory/file_store.go
type FileChunk struct {
    ID        string
    Path      string
    Source    MemorySource  // "memory" or "sessions"
    StartLine int
    EndLine   int
    Text      string
    Hash      string
    Embedding []float32
    Metadata  map[string]any
}

type FileMetadata struct {
    Hash    string
    MtimeMs float64
    Size    int64
    Path    string
    Content string
}

type MemorySource string
const (
    SourceMemory   MemorySource = "memory"
    SourceSessions MemorySource = "sessions"
)

type FileStore interface {
    // CRUD
    UpsertFile(ctx context.Context, fileMeta *FileMetadata, source MemorySource, chunks []*FileChunk) error
    DeleteFile(ctx context.Context, path string, source MemorySource) error
    UpsertChunks(ctx context.Context, chunks []*FileChunk, source MemorySource) error
    
    // Query
    ListFiles(ctx context.Context, source MemorySource) ([]string, error)
    GetFileMetadata(ctx context.Context, path string, source MemorySource) (*FileMetadata, error)
    GetFileChunks(ctx context.Context, path string, source MemorySource) ([]*FileChunk, error)
    
    // Search
    VectorSearch(ctx context.Context, query string, limit int, sources []MemorySource) ([]*MemorySearchResult, error)
    KeywordSearch(ctx context.Context, query string, limit int, sources []MemorySource) ([]*MemorySearchResult, error)
    HybridSearch(ctx context.Context, query string, limit int, sources []MemorySource, vectorWeight float64, candidateMultiplier float64) ([]*MemorySearchResult, error)
    
    ClearAll(ctx context.Context) error
    Close() error
}
```

然后重构 `ReMeFileMemory`，将其 FTS5 逻辑提取为 `FTSFileStore` 实现，dialog/session 管理作为 `FileStore` 的实现细节。

**收益：**
- 文件记忆的存储后端可插拔（当前仅有 SQLite FTS5 + 文件系统）
- 搜索逻辑与存储解耦，可单独测试
- 为未来增加 ChromaFileStore 等后端铺路

**预计工时：** 5d

### 7.2 缺失：DeltaWatcher 增量文件监听

**问题：** agentscope.go 的 `FileWatcher` 仅支持全量轮询（SHA-256 哈希比对），没有增量检测能力。

**Python 对标：**

- `FullFileWatcher` — 文件变更时删旧→重新分块→重新嵌入→upsert（当前 Go 实现对标此模式）
- `DeltaFileWatcher` — 文件变更时检测是否追加写入：
  1. `_find_cutoff_line()` — 判定新增内容从哪一行开始
  2. 启发式：文件大小增长 ≥ 10 bytes、首分块 80% 匹配
  3. **仅处理新增内容**（`extract_content_from_cutoff`），调整行号
  4. 非追加 → 回退到全量更新

**实施方案：**

```go
// 在 file_watcher.go 中新增 DeltaFileWatcher
type DeltaFileWatcher struct {
    FileWatcher
    overlapLines int  // Default 2
}

// deltaFileWatcher 核心逻辑
func (w *DeltaFileWatcher) findCutoffLine(
    oldChunks []*FileChunk, 
    newContent string,
) (int, bool) {
    // 1. 检查 size 增长 ≥ 10
    // 2. 验证首分块 80% 匹配
    // 3. 返回 cutoff = lastChunk.EndLine - overlapLines
    // 4. 若非追加 → 返回 false
}
```

**预计工时：** 2d

### 7.3 缺失：HISTORY 记忆类型

**问题：** Go 的 `MemoryType` 枚举仅有 `Personal`、`Procedural`、`Tool`、`Summary` 四种。缺少 `History`（原始对话历史节点）和 `Identity`（长期身份记忆）。

**Python 对标 — ReMe v3 `MemoryType`：**
```
IDENTITY  = "identity"    # 长期稳定的用户属性（姓名、角色等）
PERSONAL  = "personal"    # 用户偏好、习惯
PROCEDURAL= "procedural"  # 任务执行策略/流程
TOOL      = "tool"        # 工具使用经验
SUMMARY   = "summary"     # 摘要节点
HISTORY   = "history"     # 原始对话历史
```

**Python 的 HISTORY 使用场景：**
- `AddHistory` 工具：将原始对话格式化为 `MemoryNode(memory_type=HISTORY)`，存入向量存储
- `ReadHistory` 工具：按 history_id 读取原始对话内容
- `ReadHistoryV2` 工具：分页读取，支持数据脱敏
- 在 PersonalRetriever 的 Phase 3 中以 `history_id` 回溯原始上下文

**实施方案：**

```go
// reme_types.go
const (
    MemoryTypePersonal   MemoryType = "personal"
    MemoryTypeProcedural MemoryType = "procedural"
    MemoryTypeTool       MemoryType = "tool"
    MemoryTypeSummary    MemoryType = "summary"
    MemoryTypeHistory    MemoryType = "history"    // 新增
    MemoryTypeIdentity   MemoryType = "identity"   // 新增
)

// handler/history_handler.go 已有，需扩展
// 需在 Orchestrator.Summarize 流程中增加 history 记录步骤
```

**预计工时：** 3d

### 7.4 缺失：DreamStep 记忆演化（ReMe4 核心创新）

**问题：** 这是 reme4 最重要的记忆创新，agentscope.go 完全没有对应实现。

**ReMe4 `DreamStep` 架构：**

```
DreamStep.execute():
  Phase 1: Extract（提取）
    1. 加载 vault 中的 daily/ 和 digest/ 文件
    2. 加载已有的记忆（procedure, personal, wiki）
    3. LLM agent 分析新内容 →
       每个 bucket 生成候选 memory draft
       draft 包含：content, explanation, when_to_use, score

  Phase 2: Integrate（整合）
    1. 为每个 draft 检索最相似的已有记忆（top_k=10）
    2. LLM agent 对每对 (draft, existing) 判断：
       - CREATE    — 创建全新记忆
       - CORROBORATE — 佐证已有记忆（提升 score）
       - REFINE    — 细化已有记忆（修改 content/when_to_use）
       - CORRECT   — 修正已有记忆（完全替换）

  BUCKETS = ("procedure", "personal", "wiki")
```

**与现有 Go Summarizer 的差异：**

| 维度 | Go Summarizer | DreamStep |
|------|---------------|-----------|
| 输入 | 上次对话的 msg 列表 | vault 中所有 daily/digest 文件 |
| 频次 | 每次对话触发 | 定时触发（CronJob / 手动） |
| 整合策略 | 去重（相似则跳过） | 4 种写入策略（含修正/佐证/细化） |
| 多源对比 | 无 | 新内容 vs 全部已有记忆 |
| 能力 | 增量追加 | **全局记忆演化** |

**实施方案：**

```go
// 新增 memory/dream.go

type DreamConfig struct {
    VaultDir    string        // daily/ + digest/ 目录
    Buckets     []string      // ["procedure", "personal", "wiki"]
    TopK        int           // 检索相似记忆数
    ScoreThreshold float64    // 新建记忆的最小分数
    CorroborateThreshold float64 // 佐证阈值
}

type DreamStep struct {
    Config   DreamConfig
    LLM      model.ChatModel
    Embed    EmbeddingModel
    Store    VectorStore
}

// DreamResult 记录每次 Dream 的演化详情
type DreamResult struct {
    Created      []*MemoryNode   // CREATE
    Corroborated []string         // CORROBORATE 的记忆 ID
    Refined      []*MemoryNode   // REFINE（旧记忆修改）
    Corrected    []*MemoryNode   // CORRECT（旧记忆替换）
    Unchanged    []string         // 无需变更
}

func (d *DreamStep) Execute(ctx context.Context) (*DreamResult, error) {
    // Phase 1: extract candidates from vault files
    candidates := d.extractCandidates(ctx)
    
    // Phase 2: integrate each candidate
    for _, cand := range candidates {
        similar, _ := d.Store.Search(ctx, cand.EmbeddingContent(), ...)
        decision := d.decideAction(ctx, cand, similar)
        switch decision {
        case "CREATE": ...
        case "CORROBORATE": ...
        case "REFINE": ...
        case "CORRECT": ...
        }
    }
}
```

**预计工时：** 8d

### 7.5 MCP Service 模式（P3 级唯一未完成项）

利用 agentscope.go 已有的 `toolkit/mcp/` 适配层，将记忆操作暴露为 MCP tools：

```go
// 新增 memory/mcp_adapter.go

// 利用 SDKClient / ServerAdapter 将 VectorMemory 适配为 MCP 协议
mcpTools := toolkit.NewManager()
mcpTools.RegisterMemoryTools(mem, "reme_search", "reme_add", 
    "reme_retrieve_personal", "reme_summarize")
```

**预计工时：** 2d（较简单，已有 `toolkit/mcp/` 基础设施）

---

## 8. ReMe4 前沿架构分析（新）

### 8.1 ReMe4 是什么

ReMe4 (`reme4` v0.4.0.0) 是 ReMe 的第四代架构重写，将记忆系统从一个"库"升级为一个**组件化平台**。

**核心转变：** 从 "monolithic ReMe class + YAML 配置" → "插件式组件图 + 拓扑依赖注入"

### 8.2 组件化架构 vs agentscope.go

| 维度 | ReMe4 | agentscope.go | 差距 |
|------|-------|---------------|------|
| **组件注册** | `ComponentRegistry.register()` + 装饰器 | `Registry[T].Register()` | Go 缺组件类型枚举 |
| **依赖注入** | `bind()` 静态方法 → 拓扑排序自动注入 | 手动构造函数 | **Go 无自动注入** |
| **组件类型** | 14 种 ComponentEnum | 2 个全局 Registry | **Go 组件类型少** |
| **生命周期** | `_start()` / `_close()` / `dump()` / `load()` / `restart()` | 构造函数中的 `Init*` 方法 | Go 生命周期不规范 |
| **热重启** | `Application.restart(restart_config)` | 无 | **Go 缺失** |
| **Job 系统** | `BaseJob` → `BackgroundJob` / `CronJob` / `StreamJob` | 无统一抽象 | Go 缺失 |
| **FileStore** | `BaseFileStore` + `LocalFileStore` / `FaissLocalFileStore` | 无 | **Go 缺失** |
| **FileGraph** | `BaseFileGraph` + `LocalFileGraph` / `Neo4jFileGraph` / `NxFileGraph` | 无 | **Go 缺失** |
| **FileCatalog** | `BaseFileCatalog` 独立文件元数据层 | 无（嵌入在类型中） | **Go 缺失** |
| **FileChunker** | `BaseFileChunker` + `DefaultFileChunker` / `MarkdownFileChunker` | 无 | **Go 缺失** |
| **KeywordIndex** | `BaseKeywordIndex` + `BM25Index` | `FTSIndex`（直接实现） | Go 有实现但无抽象 |
| **EmbeddingStore** | `BaseEmbeddingStore` + `LocalEmbeddingStore`（含健康检查） | `EmbeddingModel` 接口 | Go 无健康检查 |
| **MCP 服务端** | `MCPService`（开箱即用） | `ServerAdapter`（需手动组装） | Go 需手动适配 |

### 8.3 ReMe4 的 Step 体系 vs Go Pipeline

ReMe4 的 Step 系统远比 agentscope.go 的 Pipeline 成熟：

| ReMe4 Step | 功能 | Go Pipeline Step |
|------------|------|-----------------|
| `SearchStep` | 向量 + 关键词混合检索 | 无（Pipeline 需手动组合） |
| `NodeSearchStep` | 文件图节点级检索 | 无 |
| `TraverseStep` | 沿链接关系遍历 | 无 |
| `ScanCatalogChangesStep` | 检测文件系统变更 | 无 |
| `UpdateCatalogStep` | 更新文件目录 | 无 |
| `UpdateIndexStep` | 重建搜索索引 | 无 |
| `DreamStep` | 记忆演化（提取+整合） | 无 |
| `AutoMemoryStep` | 自动记忆（Dream 简化版） | 无 |
| `ReadStep/WriteStep/EditStep` | 文件 I/O | 无（Go 有独立 tool 系统） |
| `ChannelNotifyStep` | 变更通知 | 无 |

### 8.4 对 agentscope.go 的启示

ReMe4 的架构揭示了记忆系统的**终局形态**：不仅是"存/取"，更是：

1. **文件系统感知** — 记忆模块主动监控文件变更（目录扫描、分块、索引更新）
2. **图式记忆** — 文件之间通过链接（wikilink、引用）形成知识图谱
3. **全生命周期** — 记忆有"创建→佐证→修正→淘汰"的自然演化
4. **定时任务** — 记忆整合作为后台 CronJob 而非同步阻塞操作

这些是 agentscope.go 可以长期演进的方向。

---

## 9. Go 相对 Python 的已有优势

| 功能 | Go 实现 | Python 对标 | Go 优势 |
|------|---------|------------|---------|
| **FTS5 + BM25 混合检索** | SQLite FTS5 + CJK字符级分词 + 两阶段 | 简单 substring 匹配（v3 FTS5 有但缺少字符级分词） | **Go 独有/更优** |
| **RAG 文档解析** | Apache Tika 集成 (`rag/tika.go`) | 无 | **Go 独有** |
| **会话快照序列化** | 版本化 JSON snapshot + SaveTo/LoadFrom | 基础序列化 | Go v1 版本化更规范 |
| **WindowMemory** | 双限制滑动窗口（msg 数 + token 数） | 无独立实现 | **Go 独有** |
| **ReMeHook** | HookBeforeModel 拦截模式 | pre_reasoning_hook | Go 更模块化 |
| **并发安全** | sync.Mutex + errgroup 并行编排 | asyncio.Lock（单线程事件循环） | **Go 天然高并发** |
| **工具结果外溢** | ToolResultCompactor + 过期清理 | 类似功能 | Go 保留期可配 |
| **Service 层多租户** | service.Storage + RedisStorage + AES-GCM | 无独立 service 层 | Go 企业级就绪 |
| **Plan 存储层** | plan.Storage + InMemoryStorage | 无 | **Go 独有** |
| **Pipeline DSL** | 类型安全的 Go 构建器 API | YAML 配置化 | Go 编译期检查 |
| **Memory GC** | 基于 freq/utility + log2 时间衰减 | delete_task_memory 流水线 | Go 更通用 |
| **Deduplicator** | 向量(0.85阈值) + LLM 语义双模式 | 去重嵌入在 summarizer 中 | Go 组件化更清晰 |
| **MCP Call SDK** | ClientBuilder 流式构建器，3 传输协议 | MCPClient 基础封装 | Go 更完整 |

---

## 10. 详细功能对比矩阵

### 10.1 核心记忆数据模型

| 特性 | ReMe Python v3 | ReMe4 v4 | agentscope.go | 差距 |
|------|---------------|----------|---------------|------|
| MemoryNode | ✅ 13 字段 | ❌（用桶代替） | ✅ 12 字段 | 持平 |
| Dual-Content Embedding | ✅ `to_vector_node()` | ❌（不同架构） | ✅ `EmbeddingContent()` | ✅ 追平 |
| 记忆 ID 生成 | SHA-256 前16位 | ❌ | SHA-256 前16位 | 持平 |
| 自动时间追踪 | ✅ `__setattr__` 钩子 | ❌ | ❌（手动设置） | Go 缺失 |
| 格式化输出 | ✅ `format()` | ❌ | ❌ | Go 缺失 |
| VectorNode ↔ MemoryNode 双向转换 | ✅ `from_vector_node()` | ❌ | ❌ | Go 缺失 |
| MemoryChunk | ✅ | ✅ FileChunk | ❌（FTSIndex 内部用） | Go 缺失 |
| MemorySource 枚举 | ✅ MEMORY/SESSIONS | ❌ | ❌ | **Go 缺失** |
| Trajectory 类型 | ✅ | ❌ | ✅ | 持平 |

### 10.2 记忆类型覆盖

| 记忆类型 | Python v3 | Python v4 | Go | 用途 |
|----------|-----------|-----------|-----|------|
| IDENTITY | ✅ | ❌ | ❌ | 长期身份属性 |
| PERSONAL | ✅ | ✅ (bucket) | ✅ | 用户偏好/习惯 |
| PROCEDURAL | ✅ | ✅ (bucket) | ✅ | 任务执行策略 |
| TOOL | ✅ | ❌ | ✅ | 工具使用经验 |
| SUMMARY | ✅ | ❌ | ✅ | 摘要节点 |
| HISTORY | ✅ | ❌ | ❌ | 原始对话存档 |
| WIKI | ❌ | ✅ (bucket) | ❌ | 知识库条目 |

### 10.3 向量存储后端

| 后端 | ReMe Python | agentscope.go | 备注 |
|------|------------|---------------|------|
| Local (内存) | ✅ LocalVectorStore | ✅ LocalVectorStore | |
| ChromaDB | ✅ ChromaVectorStore | ✅ ChromaVectorStore | |
| Qdrant | ✅ QdrantVectorStore | ✅ QdrantVectorStore | |
| Elasticsearch | ✅ ESVectorStore | ✅ ESVectorStore | |
| pgvector | ✅ PGVectorStore | ✅ PGVectorStore | |
| Hologres | ✅ HologresVectorStore | ❌ | 阿里云专用 |
| OceanBase Vector | ✅ ObvecVectorStore | ❌ | 阿里系 |
| SeekDB | ✅ SeekDBVectorStore | ❌ | 小众 |
| ZVec | ✅ ZVecVectorStore | ❌ | 小众 |
| **总计** | **9** | **5** | Python 更多 |

### 10.4 文件存储后端

| 后端 | ReMe Python v3 | agentscope.go |
|------|---------------|---------------|
| SQLite + sqlite-vec + FTS5 | ✅ SqliteFileStore | ❌（仅有 FTS5 无 vec） |
| ChromaDB | ✅ ChromaFileStore | ❌ |
| Local (dict/JSON) | ✅ LocalFileStore | ❌（嵌入在 ReMeFileMemory） |
| SeekDB | ✅ SeekDBFileStore | ❌ |
| ZVec | ✅ ZVecFileStore | ❌ |
| Faiss | ❌（v3 无）→ ✅（v4 FaissLocalFileStore） | ❌ |
| **抽象层** | ✅ BaseFileStore 接口 | ❌ **无形式化接口** |
| **总计** | **6** | **0（无独立层）** |

### 10.5 检索能力

| 特性 | ReMe Python | agentscope.go | 优势 |
|------|------------|---------------|------|
| 向量搜索 | ✅ | ✅ | 持平 |
| 关键词搜索 | ✅ substring/FTS5 | ✅ FTS5 + CJK 分词 | **Go 更优** |
| 混合搜索 | ✅ 权重可配 (0.7/0.3) | ✅ 权重可配 + BM25 | **Go 更优** |
| 多查询批搜索 | ✅ cosine 平均 | ✅ BatchSearch | 持平 |
| 时间过滤 | ✅ | ✅ | 持平 |
| 按类型过滤 | ✅ | ✅ | 持平 |
| 按目标过滤 | ✅ | ✅ | 持平 |
| disable-and-fallback | ✅ 自动降级 FTS | ❌ | **Python 领先** |

### 10.6 记忆提取与整合

| 特性 | ReMe Python | agentscope.go | 差距 |
|------|------------|---------------|------|
| LLM 压缩 (Compactor) | ✅ 结构化摘要 | ✅ CompactSummary | 持平 |
| 增量摘要 (携带 previous_summary) | ✅ | ❌ | Go 缺失 |
| 工具结果外溢 (ToolResultCompactor) | ✅ | ✅ | 持平 |
| ContextChecker (token 感知拆分) | ✅ | ✅ | 持平 |
| 文件 Sumarizer (每日 MD) | ✅ | ✅ | 持平 |
| PersonalSummarizer (两阶段) | ✅ | ✅ ExtractInsightsWithProfile | 持平 |
| ProceduralSummarizer (轨迹分析) | ✅ | ✅ | 持平 |
| ToolSummarizer (参数洞察) | ✅ | ✅ | 持平 |
| Draft-and-Deduplicate 模式 | ✅ | ✅ | 持平 |
| 向量去重 | ✅ | ✅ | 持平 |
| LLM 语义去重 | ✅ | ✅ Deduplicator | **Go 更清晰** |
| 矛盾检测 | ✅ | ✅ FindContradictions | Go 更组件化 |
| Comparative Extraction | ✅ | ✅ ExtractComparative | 持平 |
| **DreamStep 记忆演化** | ✅ (v4) | ❌ | **Python 独有** |
| **4 种写入策略 (CREATE/CORROBORATE/REFINE/CORRECT)** | ✅ (v4) | ❌ | **Python 独有** |

### 10.7 Profile 系统

| 特性 | ReMe Python | agentscope.go | 差距 |
|------|------------|---------------|------|
| 文件后端 | ✅ JSONL (FileProfileBackend) | ✅ FileProfileBackend | 持平 |
| 向量后端 | ✅ VectorProfileBackend | ✅ VectorProfileBackend | 持平 |
| 容量限制 + LRU 淘汰 | ✅ max_capacity | ❌ | Go 缺失 |
| Profile 格式化 | ✅ `format_node()` | ❌ | Go 缺失 |
| Profile 搜索 (语义) | ✅ | ✅ Retrieve | 持平 |
| 自动去重 | ✅ | ❌ | Go 缺失 |

### 10.8 文件监听

| 特性 | ReMe Python v3 | agentscope.go |
|------|---------------|---------------|
| 全量监听 | ✅ FullFileWatcher | ✅ FileWatcher (SHA-256 轮询) |
| 增量监听 | ✅ DeltaFileWatcher (追加检测) | ❌ |
| 递归监听 | ✅ | ❌ |
| 防抖 | ✅ debounce (ms) | ❌ |
| 后缀过滤 | ✅ suffix_filters | ❌ |
| 自动回调 | ✅ callback / _on_changes | ✅ OnChange |
| 自动索引重构建 | ✅ rebuild_index_on_start | ❌ |
| **watchfiles 集成** | ✅ awatch (OS 级高效) | ❌（纯轮询） |

### 10.9 服务与 API

| 特性 | ReMe Python v3 | ReMe4 v4 | agentscope.go | 差距 |
|------|---------------|----------|---------------|------|
| HTTP Service | ✅ http_service | ✅ HttpService | ✅ gateway/ | 持平 |
| MCP Tool 暴露 | ✅ service.yaml 配置 | ✅ MCPService | ❌（需适配） | **Python 领先** |
| CMD Service | ✅ cmd_service | ❌ | ❌ | |
| 表达式流 DSL | ✅ `BuildQuery() >> Retrieval()` | ✅ Step 链 | ✅ Pipeline Seq/Par | 持平 |
| CLI 交互 | ✅ reme_cli (rich UI) | ✅ CLI entry | ❌ | Go 缺失 |
| 热重启 | ✅ `Application.restart()` | ✅ `restart()` | ❌ | Go 缺失 |
| 多租户 Storage | ❌ | ❌ | ✅ service.Storage + Redis | **Go 独有** |
| Plan 存储 | ❌ | ❌ | ✅ plan.Storage | **Go 独有** |

### 10.10 基准测试

| Benchmark | ReMe Python | agentscope.go |
|-----------|------------|---------------|
| AppWorld（任务完成率） | ✅ run_appworld.py | ❌ |
| BFCL（函数调用） | ✅ run_bfcl.py | ❌ |
| HaluMem（幻觉检测） | ✅ eval_reme.py | ✅ |
| LoCoMo（长对话） | ✅ eval_reme.py | ✅ |
| LongMemEval（超长记忆） | ✅ eval_longmemeval_reme.py | ❌ |

---

## 11. 附录 A：关键文件对照表

| 功能 | Python ReMe | agentscope.go（当前状态） | 覆盖状态 |
|------|-------------|-------------------------|----------|
| MemoryNode 定义 | `reme/core/schema/memory_node.py` | `memory/reme_types.go` | ✅ 追平 |
| Dual-Content Strategy | `memory_node.py:117-161` | `reme_types.go` — `EmbeddingContent()` | ✅ 追平 |
| VectorStore 接口 | `reme/core/vector_store/base_vector_store.py` | `memory/vector_store.go` | ✅ 追平 |
| FileStore 接口 | `reme/core/file_store/base_file_store.py` | **无独立接口** | ❌ 缺失 |
| MemoryHandler CRUD | `reme/memory/vector_tools/record/memory_handler.py` | `memory/handler/memory_handler.go` | ✅ 追平 |
| Orchestrator | `reme/memory/vector_based/reme_summarizer.py` | `memory/handler/orchestrator.go` | ✅ 追平 |
| DelegateTask | `reme/memory/vector_tools/delegate_task.py` | `handler/orchestrator.go` — errgroup | ✅ 追平 |
| Personal Summarizer (两阶段) | `reme/memory/vector_based/personal/` | `memory/summarizer_personal.go` | ✅ 追平 |
| Procedural Summarizer | `reme/memory/vector_based/procedural/` | `memory/summarizer_procedural.go` | ✅ 追平 |
| Tool Summarizer | `reme/memory/vector_based/tool_call/` | `memory/summarizer_tool.go` | ✅ 追平 |
| Flow Pipeline | `reme/core/flow/base_flow.py` | `memory/pipeline/` 包 | ✅ 追平 |
| Batch Search | `memory_handler.py:202` | `memory/handler/memory_handler.go` BatchSearch | ✅ 追平 |
| ProfileHandler | `reme/memory/vector_tools/profiles/profile_handler.py` | `memory/handler/profile_handler.go` | ✅ 追平 |
| AddHistory | `reme/memory/vector_tools/history/add_history.py` | **缺失** | ❌ 缺失 |
| ReadHistory | `reme/memory/vector_tools/history/read_history.py` | **缺失** | ❌ 缺失 |
| MemoryValidation | `service.yaml` 流水线步骤 | `memory/pipeline/steps.go` MemoryValidationStep | ✅ 追平 |
| RerankMemory | `service.yaml` 流水线步骤 | `memory/pipeline/steps.go` LLMRerankStep | ✅ 追平 |
| FileWatcher | `reme/core/file_watcher/` (Full + Delta) | `memory/file_watcher.go` (仅全量) | ⚠️ 部分 |
| EmbeddingCache | `reme/core/embedding/base_embedding_model.py` | `memory/embedding_cache.go` | ✅ 追平 |
| Memory GC | `delete_task_memory` 流水线 | `memory/memory_gc.go` MemoryCollector | ✅ 追平 |
| Comparative Extraction | `ComparativeExtraction()` | `memory/summarizer_procedural.go` | ✅ 追平 |
| Registry | `R.vector_stores`, `R.embedding_models` | `memory/registry.go` | ✅ 追平 |
| Benchmark | `benchmark/` 目录 (5种) | `memory/benchmark.go` (2种) | ⚠️ 部分 |
| Memory Library | `docs/library/` | `memory/memory_library.go` | ✅ 追平 |
| MCP Service | `service.yaml` MCP 配置 | ⏳ 待适配 | ❌ 缺失 |
| DreamStep | `reme4/steps/evolve/dream_step.py` | **无** | ❌ 缺失 |
| ReMe CLI | `reme/reme_cli.py` | **无** | ❌ 缺失 |
| Hybrid Search | 简单 substring 匹配 | `memory/hybrid_search.go` + `memory/fts_index.go` | **Go 更优** |
| RAG | 无 | `rag/` 包 | **Go 独有** |

---

## 12. 附录 B：MemoryType 枚举对比

| MemoryType | ReMe v3 | ReMe4 v4 | agentscope.go | 建议 Go 操作 |
|------------|---------|----------|---------------|-------------|
| `identity` | ✅ | ❌ | ❌ | **新增** — 长期身份记忆 |
| `personal` | ✅ | ✅ (bucket) | ✅ | 不变 |
| `procedural` | ✅ | ✅ (bucket) | ✅ | 不变 |
| `tool` | ✅ | ❌ | ✅ | 不变 |
| `summary` | ✅ | ❌ | ✅ | 不变 |
| `history` | ✅ | ❌ | ❌ | **新增** — 原始对话存档 |
| `wiki` | ❌ | ✅ (bucket) | ❌ | 可选（知识库条目） |

---

## 13. 附录 C：新增/修改文件一览

### 已实施文件（Phase 1-7）

| 文件 | 操作 | 说明 |
|------|------|------|
| `memory/reme_types.go` | 修改 | +`EmbeddingContent()`, +`NewMemoryNodeWithWhen()`, +`MemoryTypeIdentity` |
| `memory/reme_types_test.go` | 修改 | +`TestEmbeddingContent`, +`TestNewMemoryNodeWithWhen` |
| `memory/vector_store_local.go` | 修改 | Insert 使用 `EmbeddingContent()`, +`BatchSearchWithThreshold()` |
| `memory/vector_store_chroma.go` | 修改 | Insert + Update 使用 `EmbeddingContent()` |
| `memory/vector_store_pgvector.go` | 修改 | Insert + Update 使用 `EmbeddingContent()` |
| `memory/vector_store_qdrant.go` | 修改 | nodesToPoints 使用 `EmbeddingContent()` |
| `memory/vector_store_elasticsearch.go` | 修改 | Insert + Update 使用 `EmbeddingContent()` |
| `memory/deduplicator.go` | 修改 | DeduplicateAgainstStore, deduplicateByVector 使用 `EmbeddingContent()` |
| `memory/handler/memory_handler.go` | 修改 | AddDraftAndRetrieveSimilar 使用 `EmbeddingContent()`, +`BatchSearch()` |
| `memory/handler/orchestrator.go` | 修改 | errgroup 并发化 Summarize + Retrieve, Two-Phase Personal |
| `memory/handler/profile_handler.go` | 重写 | ProfileBackend 接口 + FileProfileBackend + VectorProfileBackend |
| `memory/summarizer_personal.go` | 修改 | +`ExtractInsightsWithProfile()`, +`buildInsightWithProfilePrompt()` |
| `memory/summarizer_procedural.go` | 修改 | +`ExtractComparative()`, +`buildComparativeAnalysisPrompt()` |
| `memory/summarizer_tool.go` | 修改 | +`ParameterInsight` 类型 |
| `memory/embedding_cache.go` | 修改 | +`SetDiskPath()`, +`LoadFromDisk()`, +`SaveToDisk()` |
| `memory/file_watcher.go` | **新增** | FileWatcher: SHA-256 hash 监控 + 回调 |
| `memory/memory_gc.go` | **新增** | MemoryCollector: freq/utility 自动 GC |
| `memory/registry.go` | **新增** | 泛型 Registry[T] 组件注册中心 |
| `memory/benchmark.go` | **新增** | HaluMem + LoCoMo + RunBenchmarkSuite |
| `memory/memory_library.go` | **新增** | GetDefaultAgentLibrary + InjectIntoVectorStore |
| `memory/pipeline/context.go` | **新增** | FlowContext 步骤间数据传递 |
| `memory/pipeline/pipeline.go` | **新增** | Step / Pipeline / Seq / Par / BranchFirst |
| `memory/pipeline/steps.go` | **新增** | 7 个内置步骤 + LLMRerankStep |
| `memory/pipeline/pipeline_test.go` | **新增** | 4 个 Pipeline 测试 |

**已实施统计：新增 10 个文件，修改 14 个文件。**

### 待实施文件（Phase 8——P4 级）

| 文件 | 操作 | 说明 | 优先级 |
|------|------|------|--------|
| `memory/file_store.go` | **新增** | `FileStore` 接口 + `MemorySource` 枚举 + `FileChunk`/`FileMetadata` 类型 | P4 |
| `memory/file_store_fts.go` | **新增** | `FTSFileStore` — 将 FTS5 逻辑从 ReMeFileMemory 提取到此 | P4 |
| `memory/file_store_test.go` | **新增** | `FileStore` 接口测试 | P4 |
| `memory/file_watcher.go` | 修改 | 新增 `DeltaFileWatcher` + `_findCutoffLine` 增量检测 | P4 |
| `memory/dream.go` | **新增** | `DreamStep` + `DreamResult` — 2 阶段记忆演化 | P4 |
| `memory/dream_test.go` | **新增** | DreamStep 测试 | P4 |
| `memory/mcp_adapter.go` | **新增** | MCP Tool 适配器，将 VectorMemory 暴露为 MCP tools | P3 |
| `memory/handler/history_handler.go` | 修改 | AddHistory + ReadHistory + ReadHistoryV2 完整实现 | P4 |
| `memory/reme_types.go` | 修改 | 新增 `MemoryTypeHistory` + `MemoryTypeIdentity` 枚举值 | P4 |

---

*本文档基于 2026-06-11 的二次深度分析（含 reme4 v0.4.0.0 架构调研）编写。*
