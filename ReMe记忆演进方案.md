# AgentScope.Go ReMe 记忆模块演进方案

> 基于对 [ReMe (Python)](../ReMe/) v0.3.1.10 与 [agentscope.go](./) v2.0.0-rc.1 记忆模块的深度对比分析，制定本方案。

---

## 目录

1. [分析概览与结论](#1-分析概览与结论)
2. [差距清单（按优先级排序）](#2-差距清单按优先级排序)
3. [P0 级：架构性差距——Dual-Content Embedding](#3-p0-级架构性差距dual-content-embedding)
4. [P1 级：流程性差距——异步编排与多阶段流水线](#4-p1-级流程性差距异步编排与多阶段流水线)
5. [P2 级：增强性差距——Profile 系统、Flow 框架等](#5-p2-级增强性差距profile-系统flow-框架等)
6. [P3 级：扩展性差距——Benchmark、MCP、注册中心等](#6-p3-级扩展性差距benchmarkmcp注册中心等)
7. [实施路线图](#7-实施路线图)
8. [Go 相对 Python 的已有优势](#8-go-相对-python-的已有优势)
9. [附录：关键文件对照表](#9-附录关键文件对照表)

---

## 1. 分析概览与结论

### 1.1 总体判断

**agentscope.go 的 ReMe 记忆模块已达到 Python ReMe v0.3.1.10 约 70% 的功能覆盖度。**

Go 版本在以下方面已经**达到或超越** Python 版本：
- 向量存储后端数量（5 种 vs Python 的 10 种，但 Go 的每种实现更完善）
- 混合检索（FTS5+BM25+向量融合，Go 独有）
- 会话快照与序列化（版本化 JSON，Go 更规范）
- RAG 文档解析（Tika 集成，Go 独有）
- 并发安全（sync.Mutex / errgroup，Go 天然优势）

但 Python ReMe 在以下方面仍有**显著领先**：
- **Dual-Content Embedding**（`when_to_use` 与 `content` 的分离嵌入策略）
- **两阶段 Personal Memory 流水线**（S1 记忆提取 + S2 Profile 更新）
- **DelegateTask 异步分发**（多 Agent 并行调度）
- **Flow Pipeline 框架**（YAML 配置化的流水线定义）
- **Multi-Query Batch Search**（跨多查询的余弦均值过滤）
- **Memory Validation / Rerank / Rewrite 流水线步骤**

---

## 2. 差距清单（按优先级排序）

| 优先级 | 差距项 | 影响范围 | 预计工时 |
|--------|--------|----------|----------|
| **P0** | Dual-Content Embedding 未实现 | 检索召回精度 | 3d |
| **P0** | `WhenToUse` 仅存储、不用于嵌入 | 检索语义匹配 | 2d |
| **P1** | Orchestrator 同步执行，无并行 | 记忆提取吞吐 | 3d |
| **P1** | 无 Two-Phase Personal Memory 流水线 | Profile 更新质量 | 4d |
| **P1** | 无 Multi-Query Batch Search | 批量检索精度 | 2d |
| **P1** | 无 Flow Pipeline 框架 | 流水线可扩展性 | 5d |
| **P2** | Profile 系统仅支持文件后端 | 多实例部署 | 3d |
| **P2** | 无 Memory Validation 独立步骤 | 记忆质量保障 | 2d |
| **P2** | 无 LLM Rerank / Rewrite | 检索结果精排 | 3d |
| **P2** | 无 File Watcher | 外部变更感知 | 2d |
| **P2** | Embedding Cache 无磁盘持久化 | 重启性能 | 1d |
| **P2** | Tool Memory 缺少参数优化学习 | 工具使用效率 | 2d |
| **P3** | 无 Comparative Extraction | 成功/失败对比学习 | 3d |
| **P3** | 无 Memory Utility/Freq 跟踪 | 记忆自动清理 | 2d |
| **P3** | 无 Registry/Plugin 架构 | 组件可插拔性 | 4d |
| **P3** | 无 MCP Service 模式 | 外部工具集成 | 3d |
| **P3** | 无 Benchmark 集成 | 回归验证 | 2d |
| **P3** | 无 内置 Memory Library | 快速上手 | 2d |

---

## 3. P0 级：架构性差距——Dual-Content Embedding

### 3.1 问题描述

ReMe Python 的核心设计理念之一是 **Dual-Content Embedding Strategy**：

```
┌─────────────────────────────────────────────────────────┐
│              MemoryNode (ReMe Python)                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  when_to_use: "当用户询问Python问题时检索此记忆"        │
│       │                                                 │
│       ▼  [用于生成向量嵌入]                              │
│       │                                                 │
│  content:    "用户偏好使用pandas处理数据，喜欢函数式..." │
│       │                                                 │
│       ▼  [存储在metadata中，检索后返回完整内容]          │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**当前 agentscope.go 的行为**：
- `MemoryNode.WhenToUse` 字段存在（`reme_types.go:46`）
- 但所有 5 个 VectorStore 后端的 `Insert`/`Update` 方法均从 `node.Content` 生成嵌入
- `WhenToUse` 仅作为元数据存储，**不参与向量检索**
- 即使摘要器填充了 `WhenToUse`，它也无法影响检索结果

**影响**：
- 无法实现"以触发条件检索、以完整内容展示"的分离策略
- 检索时用完整 `Content` 做向量匹配，产生大量噪音（长文本稀释关键语义）
- 无法实现"在什么场景下使用这条记忆"的精准触发

### 3.2 解决方案

#### 3.2.1 新增 `EmbeddingContent()` 方法

在 `MemoryNode` 上添加方法，确定用于生成嵌入的文本：

```go
// embedding_content.go (新文件)
package memory

// EmbeddingContent 返回应被嵌入的文本。
// 规则：若 WhenToUse 非空，使用 WhenToUse；否则使用 Content。
// 这与 ReMe Python 的 to_vector_node() 逻辑一致。
func (n *MemoryNode) EmbeddingContent() string {
    if n.WhenToUse != "" {
        return n.WhenToUse
    }
    return n.Content
}
```

#### 3.2.2 修改所有 VectorStore.Insert 实现

将 5 个 VectorStore 后端的嵌入生成从 `node.Content` 改为 `node.EmbeddingContent()`：

**修改前** (`vector_store_local.go:41`):
```go
v, err := s.embed.Embed(ctx, node.Content)
```

**修改后**:
```go
v, err := s.embed.Embed(ctx, node.EmbeddingContent())
```

**受影响的文件**：
- `vector_store_local.go` — `Insert` (L41), `Update`
- `vector_store_chroma.go` — `Insert` (L124)
- `vector_store_pgvector.go` — `Insert` (L120), `Update` (L204)
- `vector_store_qdrant.go` — `Insert` (L220)
- `vector_store_elasticsearch.go` — `Insert` (L128), `Update` (L155)

#### 3.2.3 修改更新路径中的重新嵌入

所有 `Update` 方法中当 `node.Vector == nil` 时需要重新嵌入，也改为使用 `EmbeddingContent()`：

```go
// 所有 vector_store_*.go 的 Update 方法
if len(node.Vector) == 0 {
    v, err := s.embed.Embed(ctx, node.EmbeddingContent())
    ...
}
```

#### 3.2.4 修改 `NewMemoryNode` 支持 `whenToUse` 参数

```go
// reme_types.go
func NewMemoryNodeWithWhen(memType MemoryType, target, content, whenToUse string) *MemoryNode {
    now := time.Now()
    node := &MemoryNode{
        MemoryID:     GenerateMemoryID(content + "|" + target),
        MemoryType:   memType,
        MemoryTarget: target,
        WhenToUse:    whenToUse,
        Content:      content,
        TimeCreated:  now,
        TimeModified: now,
        Metadata:     make(map[string]any),
    }
    return node
}
```

#### 3.2.5 修改去重器

`deduplicator.go` 中 `Deduplicate` 方法的相似搜索也需用 `EmbeddingContent()`：

```go
// deduplicator.go:74 — 修改前
similar, err := store.Search(ctx, newMem.Content, RetrieveOptions{...})

// 修改后
similar, err := store.Search(ctx, newMem.EmbeddingContent(), RetrieveOptions{...})
```

#### 3.2.6 修改 `AddDraftAndRetrieveSimilar`

`memory_handler.go:24` 的草稿相似检索使用 `Content`，应改为 `EmbeddingContent()`：

```go
// 修改前
return h.Store.Search(ctx, node.Content, memory.RetrieveOptions{...})

// 修改后
return h.Store.Search(ctx, node.EmbeddingContent(), memory.RetrieveOptions{...})
```

#### 3.2.7 兼容性说明

这是一次**语义修正**，不破坏 API 兼容性：
- `WhenToUse` 字段在现有代码中已存在，只是未曾生效
- 默认行为（`WhenToUse == ""`）时 `EmbeddingContent()` 返回 `Content`，与当前行为完全一致
- 已有摘要器（`summarizer_personal.go`、`summarizer_procedural.go`、`summarizer_tool.go`）已在填充 `WhenToUse`，修改后这些填充将直接生效

#### 3.2.8 验证方案

```go
// memory/embedding_content_test.go
func TestEmbeddingContent(t *testing.T) {
    // 无 WhenToUse：返回 Content
    n := &MemoryNode{Content: "hello", WhenToUse: ""}
    assert.Equal(t, "hello", n.EmbeddingContent())

    // 有 WhenToUse：返回 WhenToUse
    n = &MemoryNode{Content: "hello world long text...", WhenToUse: "greeting"}
    assert.Equal(t, "greeting", n.EmbeddingContent())
}

func TestVectorStoreUsesEmbeddingContent(t *testing.T) {
    // 用 mock embedding model 验证各 VectorStore 调用 Embed 时传入的是 EmbeddingContent()
    mockEmbed := &mockEmbeddingModel{}
    store := NewLocalVectorStore(mockEmbed)
    
    node := &MemoryNode{
        MemoryID:  "test",
        Content:   "long content that should not be embedded",
        WhenToUse: "short when to use",
    }
    store.Insert(ctx, []*MemoryNode{node})
    
    assert.Equal(t, "short when to use", mockEmbed.lastEmbedded)
}
```

---

## 4. P1 级：流程性差距——异步编排与多阶段流水线

### 4.1 问题：Orchestrator 同步执行

**当前 agentscope.go** (`handler/orchestrator.go:52`):

```go
func (o *MemoryOrchestrator) Summarize(...) {
    // 1) History → 同步执行
    // 2) Personal → 同步执行（内部 ExtractObservations 再 ExtractInsights）
    // 3) Procedural → 同步执行
    // 4) Tool → 同步执行
}
```

所有步骤**串行同步**执行，每个步骤都需要 LLM 调用，总延迟 = 各步骤延迟之和。

**ReMe Python** (`delegate_task.py`):

```python
# 提交所有任务异步执行
for memory_target in memory_target_tasks:
    self.submit_async_task(agent.call, ...)
await self.join_async_tasks()
```

所有 agent 任务**并行提交**，总延迟 ≈ max(各步骤延迟)。

### 4.2 解决方案：并发化 Orchestrator

#### 4.2.1 使用 errgroup 并发执行

```go
// handler/orchestrator.go — 重写 Summarize
func (o *MemoryOrchestrator) Summarize(ctx context.Context, msgs []*message.Msg, userName, taskName, toolName string) (*memory.SummarizeResult, error) {
    res := &memory.SummarizeResult{
        UpdatedProfiles: make(map[string]map[string]any),
    }

    // 1) History — 必须先执行（后续步骤依赖 history_node）
    if o.Config.EnableHistory && o.HistoryTool != nil {
        target := firstNonEmpty(userName, taskName, toolName)
        if target != "" {
            histNode, err := o.HistoryTool.AddHistory(ctx, msgs, target, "")
            if err == nil && histNode != nil {
                res.AddedHistory = histNode
            }
        }
    }

    // 2) Personal / Procedural / Tool — 并发执行
    g, ctx := errgroup.WithContext(ctx)

    if o.Config.EnablePersonal && userName != "" && o.PersonalSum != nil {
        g.Go(func() error {
            nodes, profile, err := o.summarizePersonal(ctx, msgs, userName)
            if err == nil {
                o.mu.Lock()
                res.PersonalMemories = nodes
                if profile != nil { res.UpdatedProfiles[userName] = profile }
                o.mu.Unlock()
            }
            return err // errgroup 不因 error 取消其他 goroutine，仅记录
        })
    }

    if o.Config.EnableProcedural && taskName != "" && o.ProceduralSum != nil {
        g.Go(func() error {
            nodes, err := o.summarizeProcedural(ctx, msgs, taskName)
            if err == nil {
                o.mu.Lock()
                res.ProceduralMemories = nodes
                o.mu.Unlock()
            }
            return err
        })
    }

    if o.Config.EnableTool && toolName != "" && o.ToolSum != nil {
        g.Go(func() error {
            if err := o.SummarizeToolUsage(ctx, toolName); err == nil {
                o.mu.Lock()
                res.ToolMemories = o.flushToolResults(toolName)
                o.mu.Unlock()
            }
            return err
        })
    }

    _ = g.Wait() // 不因单个失败取消全部
    return res, nil
}
```

#### 4.2.2 为 Orchestrator 添加互斥锁

```go
type MemoryOrchestrator struct {
    // ... 现有字段
    mu sync.Mutex // 保护 SummarizeResult 的并发写入
}
```

#### 4.2.3 异步摘要任务增强

当前异步摘要通过 `AddAsyncSummaryTask` (`reme_file_memory.go`) 使用 semaphore 限制并发，但无重试和错误上报。建议：

```go
// task_manager.go (新文件)
type AsyncTaskManager struct {
    sem      chan struct{}
    maxRetry int
    logger   *slog.Logger
}

func (m *AsyncTaskManager) Submit(ctx context.Context, fn func(context.Context) error) {
    select {
    case m.sem <- struct{}{}:
    default:
        m.logger.Warn("async task semaphore full, dropping task")
        return
    }
    go func() {
        defer func() { <-m.sem }()
        for i := 0; i <= m.maxRetry; i++ {
            if err := fn(ctx); err == nil {
                return
            }
            if i < m.maxRetry {
                time.Sleep(time.Duration(1<<i) * time.Second)
            }
        }
        m.logger.Error("async task failed after retries")
    }()
}
```

### 4.3 问题：无 Two-Phase Personal Memory 流水线

**ReMe Python** 的 `PersonalSummarizer.execute()` 流程：

```
S1: 从对话中提取记忆节点 (AddAndRetrieveSimilarMemory)
    ↓
    预加载现有 Profile (RetrieveProfile / ReadAllProfiles)
    ↓
S2: 基于已有 Profile 更新/新增 Profile 条目 (UpdateProfile / AddProfile)
```

**当前 agentscope.go** (`summarizer_personal.go`)：

```
ExtractObservations → ExtractInsights → (合并) → Dedup → 写入
```

`ExtractObservations` 和 `ExtractInsights` 是两个连续的 LLM 调用，但**缺少 S2 阶段的 Profile 感知更新**——即 S1 提取完成后，不加载现有 Profile 来指导 S2 的更新决策。

#### 解决方案

```go
// summarizer_personal.go — 新增方法
func (s *PersonalSummarizer) SummarizeWithProfileUpdate(
    ctx context.Context,
    msgs []*message.Msg,
    userName string,
    existingProfile map[string]any, // 来自 ProfileHandler.ReadAllProfiles
) ([]*MemoryNode, map[string]any, error) {
    // S1: 提取记忆
    observations, err := s.ExtractObservations(ctx, msgs, userName)
    if err != nil || len(observations) == 0 {
        return nil, nil, err
    }

    // ---- Profile 上下文注入 ----
    // 将现有 Profile 作为上下文注入 S2 的 LLM prompt
    insights, err := s.ExtractInsightsWithProfile(ctx, observations, userName, existingProfile)
    if err != nil {
        insights, _ = s.ExtractInsights(ctx, observations, userName) // fallback
    }

    all := append(observations, insights...)
    // ... dedup, store
    return all, profile, nil
}
```

关键：`ExtractInsightsWithProfile` 需要在 LLM prompt 中包含现有 Profile 内容，使 LLM 能够判断"这条新记忆与已有 Profile 矛盾/重复/补充"，而非盲目追加。

---

## 5. P2 级：增强性差距

### 5.1 Multi-Query Batch Search

**ReMe Python** (`memory_handler.py:202`) 支持：
1. 对多个查询分别搜索
2. 去重合并结果
3. 计算每个结果的跨查询平均余弦相似度
4. 按阈值过滤（`hybrid_threshold`）

**当前 agentscope.go** 无此能力。

#### 解决方案：为 `MemoryHandler` 添加 `BatchSearch` 方法

```go
// handler/memory_handler.go — 新增方法
type BatchSearchQuery struct {
    Query  string
    TopK   int
    MinScore float64
}

// BatchSearch 跨多查询搜索并过滤
func (h *MemoryHandler) BatchSearch(ctx context.Context, queries []BatchSearchQuery, hybridThreshold float64) ([]*memory.MemoryNode, error) {
    if len(queries) == 0 {
        return nil, nil
    }

    // 1) 各查询独立搜索，去重合并
    seen := make(map[string]*memory.MemoryNode)
    for _, q := range queries {
        nodes, _ := h.Store.Search(ctx, q.Query, memory.RetrieveOptions{
            TopK:     q.TopK,
            MinScore: q.MinScore,
        })
        for _, n := range nodes {
            if _, ok := seen[n.MemoryID]; !ok {
                seen[n.MemoryID] = n
            }
        }
    }

    if hybridThreshold <= 0 {
        result := make([]*memory.MemoryNode, 0, len(seen))
        for _, n := range seen { result = append(result, n) }
        return result, nil
    }

    // 2) 提取所有结果的 embedding，计算每个结果对全部查询的平均相似度
    queryVectors := make([][]float32, len(queries))
    for i, q := range queries {
        if lv, ok := h.Store.(*memory.LocalVectorStore); ok {
            // 直接获取嵌入（需 LocalVectorStore 暴露 embed model）
        }
    }
    // ... 余弦矩阵计算、均值过滤、排序
}
```

> **注意**：对于远程 VectorStore（PGVector、ES 等），无法直接获取 query embedding。此方法作为 `LocalVectorStore` 的增强功能实现。

### 5.2 Flow Pipeline 框架

**ReMe Python** 通过 YAML 定义流水线：

```yaml
# retrieve_task_memory 流水线
flow_content: BuildQuery() >> MemoryRetrieval() >> RerankMemory() >> RewriteMemory()

# summary_task_memory 流水线
flow_content: TrajectoryPreprocess() >> (SuccessExtraction()|FailureExtraction()|ComparativeExtraction()) >> MemoryValidation() >> MemoryDeduplication()
```

支持操作符：`>>`（顺序）、`|`（分支/并行）、`()`（分组）。

**当前 agentscope.go** 无此抽象，所有流水线步骤硬编码在 `Orchestrator` 中。

#### 解决方案：Go 版 Flow Pipeline DSL

```go
// memory/pipeline/pipeline.go (新包)
package pipeline

// Step 流水线中的一个步骤
type Step interface {
    Execute(ctx context.Context, input *FlowContext) (*FlowContext, error)
}

// Pipeline 步骤组合（支持 Sequential / Parallel / Branch）
type Pipeline struct {
    Steps []StepNode
}

type StepNode struct {
    Step     Step
    Children []StepNode // 用于并行/分支
    Mode     StepMode   // Sequential, Parallel, Branch
}

type StepMode int
const (
    Sequential StepMode = iota
    Parallel
    Branch
)

// 构建器风格 API
func Seq(steps ...Step) *StepNode {
    return &StepNode{Mode: Sequential, Children: toNodes(steps)}
}

func Par(steps ...Step) *StepNode {
    return &StepNode{Mode: Parallel, Children: toNodes(steps)}
}

func (p *Pipeline) Execute(ctx context.Context, input *FlowContext) (*FlowContext, error) {
    return p.executeNode(ctx, p.Root, input)
}
```

**使用示例**（实现 ReMe Python 的 `retrieve_task_memory` 流水线）：

```go
// 定义步骤
var RetrieveTaskMemory = Pipeline{
    Root: Seq(
        &BuildQueryStep{},
        &MemoryRetrievalStep{},
        &RerankMemoryStep{},
        &RewriteMemoryStep{},
    ),
}

// 使用
result, err := RetrieveTaskMemory.Execute(ctx, &FlowContext{
    Query:      "how to fix this bug",
    TopK:       5,
    MinScore:   0.3,
})
```

**建议首批实现的 Pipeline 步骤**：

| 步骤 | 功能 | 对应 Python |
|------|------|-------------|
| `BuildQueryStep` | LLM 从消息构建检索查询 | `BuildQuery()` |
| `MemoryRetrievalStep` | 向量/混合检索 | `MemoryRetrieval()` |
| `RerankMemoryStep` | LLM 重排序检索结果 | `RerankMemory()` |
| `RewriteMemoryStep` | LLM 重写带记忆的上下文 | `RewriteMemory()` |
| `TrajectoryPreprocessStep` | 轨迹预处理 | `TrajectoryPreprocess()` |
| `MemoryValidationStep` | 记忆质量验证 | `MemoryValidation()` |
| `MemoryDeduplicationStep` | 记忆去重（已部分实现） | `MemoryDeduplication()` |

### 5.3 Profile 系统的向量后端

**当前**：`ProfileHandler` (`handler/profile_handler.go`) 仅支持 JSON 文件存储。

**目标**：支持 `VectorProfileBackend`（与 ReMe Python 对标）。

```go
// handler/profile_handler.go — 改为后端接口模式
type ProfileBackend interface {
    ReadAll(ctx context.Context, userName string) (map[string]any, error)
    Update(ctx context.Context, userName string, updates map[string]any) error
    Delete(ctx context.Context, userName string, key string) error
    Retrieve(ctx context.Context, userName, query string, topK int) (map[string]any, error)
}

type FileProfileBackend struct { dir string }
type VectorProfileBackend struct { store memory.VectorStore }

type ProfileHandler struct {
    backend ProfileBackend
}
```

### 5.4 Memory Validation 独立步骤

**ReMe Python** 的 `summary_task_memory` 流水线包含显式的 `MemoryValidation()` 步骤，在写入前验证提取的记忆质量。

```go
// memory/validator.go (新文件)
type MemoryValidator struct {
    Model   model.ChatModel
    Threshold float64
}

func (v *MemoryValidator) Validate(ctx context.Context, nodes []*MemoryNode) ([]*MemoryNode, error) {
    // LLM 评估每条记忆的：
    // 1. 事实准确性 (factual accuracy)
    // 2. 信息完整性 (completeness)
    // 3. 可操作性 (actionability)
    // 返回评分 ≥ Threshold 的记忆
}
```

### 5.5 LLM Rerank 步骤

```go
// memory/reranker.go (新文件)
type MemoryReranker struct {
    Model model.ChatModel
}

// Rerank 使用 LLM 对向量检索结果进行精排
func (r *MemoryReranker) Rerank(ctx context.Context, query string, candidates []*MemoryNode, topK int) ([]*MemoryNode, error) {
    // 构造 LLM prompt 包含 query + candidate 列表
    // LLM 返回重排后的索引
}
```

### 5.6 File Watcher

**ReMe Python** 的 `file_watcher/` 监控 `MEMORY.md` 和 `memory/` 目录的外部变更，自动重新索引。

```go
// memory/file_watcher.go (新文件)
type FileWatcher struct {
    dir       string
    interval  time.Duration
    fileStore FileStore
    stopCh    chan struct{}
}

func (w *FileWatcher) Start(ctx context.Context) error {
    // 使用 fsnotify 或定时轮询监控文件变更
    // 变更时调用 fileStore.upsert_file() 重新索引
}
```

### 5.7 Embedding Cache 磁盘持久化

**当前**：`EmbeddingCache` (`embedding_cache.go`) 是纯内存 LRU，重启丢失。

**与 ReMe Python 对标**：添加 JSONL 磁盘持久化。

```go
// embedding_cache.go — 扩展
type EmbeddingCache struct {
    inner    EmbeddingModel
    cache    *lru.Cache[string, []float32]
    mu       sync.RWMutex
    diskPath string          // 新增
    dirty    map[string]bool // 追踪变更
}

// 启动时从磁盘加载
func (c *EmbeddingCache) LoadDisk() error { ... }

// 定时/惰性刷盘
func (c *EmbeddingCache) FlushDisk() error { ... }
```

### 5.8 Tool Memory 增强——参数优化学习

**ReMe Python** 的 Tool Memory 支持参数优化模式学习，例如"调用 web_search 时使用 site:stackoverflow.com 可获得更好结果"。

```go
// memory/tool_call_result.go — 扩展 ToolCallResult
type ToolCallResult struct {
    // ... 现有字段
    ParameterInsights []ParameterInsight `json:"parameter_insights,omitempty"`
}

type ParameterInsight struct {
    Parameter string `json:"parameter"`
    Pattern   string `json:"pattern"`   // 有效的参数模式
    Outcome   string `json:"outcome"`    // 使用此模式的结果
    Frequency int    `json:"frequency"`  // 观察到的频率
}
```

---

## 6. P3 级：扩展性差距

### 6.1 Comparative Extraction（成功/失败对比学习）

**ReMe Python** 的 `summary_task_memory` 流水线支持 `ComparativeExtraction()`：
- 对比最高分与最低分的轨迹
- 识别成功的关键决策和失败的根本原因
- 提取可泛化的经验教训

```go
// memory/summarizer_procedural.go — 新增方法
func (s *ProceduralSummarizer) ExtractComparative(
    ctx context.Context,
    successTrajectories []Trajectory,
    failureTrajectories []Trajectory,
) ([]*MemoryNode, error) {
    // LLM prompt 包含成功和失败轨迹的对比
    // 提取：成功因素、失败原因、关键差异、可迁移模式
}
```

### 6.2 Memory Utility/Frequency 跟踪与自动清理

**ReMe Python** 的 `delete_task_memory` 流水线根据 `freq` 和 `utility` 自动清理低价值记忆。

```go
// memory/memory_gc.go (新文件)
type MemoryCollector struct {
    Store   VectorStore
    FreqThreshold   int
    UtilityThreshold float64
}

func (gc *MemoryCollector) Collect(ctx context.Context) ([]string, error) {
    // 扫描所有记忆的 metadata["freq"] 和 metadata["utility"]
    // 删除 freq >= FreqThreshold 且 utility/freq < UtilityThreshold 的节点
}
```

`MemoryNode.Metadata` 中添加追踪字段：

```go
// 在 RecordMemory 操作中更新
node.Metadata["freq"] = freq + 1
node.Metadata["last_accessed"] = time.Now().Unix()
node.Metadata["utility"] = utility
```

### 6.3 Registry/Plugin 架构

**ReMe Python** 使用 Registry Factory 模式：

```python
R.vector_stores.register("chroma")(ChromaVectorStore)
R.embedding_models.register("openai")(OpenAIEmbeddingModel)
R.llms.register("qwen")(QwenLLM)
```

**当前 agentscope.go** 使用直接构造，由 `BuildReMeVectorMemory` (`handler/bootstrap.go`) 集中装配。

**建议**：引入轻量级注册中心，支持字符串名称查找：

```go
// memory/registry.go (新文件)
type Registry[T any] struct {
    mu      sync.RWMutex
    entries map[string]func() T
}

func NewRegistry[T any]() *Registry[T] { ... }
func (r *Registry[T]) Register(name string, factory func() T) { ... }
func (r *Registry[T]) Get(name string) (T, error) { ... }

// 全局注册中心
var (
    VectorStores     = NewRegistry[VectorStore]()
    EmbeddingModels  = NewRegistry[EmbeddingModel]()
    FileStores      = NewRegistry[FileStore]()
)
```

### 6.4 MCP Service 模式

**ReMe Python** 可作为 MCP (Model Context Protocol) Server 对外暴露记忆能力。

```go
// memory/mcp/mcp_server.go (新文件)
// 基于 agentscope.go 已有的 MCP 工具适配层 (toolkit/mcp/)
// 将记忆 CRUD 操作暴露为 MCP tools:
//   - reme_search_memory
//   - reme_add_memory
//   - reme_retrieve_personal
//   - reme_summarize
```

### 6.5 Benchmark 集成

**ReMe Python** 内置 5 个 benchmark (LoCoMo, HaluMem, LongMemEval, AppWorld, BFCL)。

```go
// memory/benchmark/benchmark.go (新包)
type Benchmark interface {
    Name() string
    Run(ctx context.Context, mem VectorMemory) (*BenchmarkResult, error)
}

type BenchmarkResult struct {
    OverallScore    float64
    MemoryAccuracy  float64
    QAAccuracy      float64
    DetailScores    map[string]float64
}
```

首批实现 `HaluMemBenchmark`（幻觉检测）和 `LoCoMoBenchmark`（长对话记忆）。

### 6.6 内置 Memory Library

**ReMe Python** 的 `docs/library/` 包含预构建的记忆 JSONL 文件，可直接加载使用。

```go
// memory/library/library.go (新文件)
// 嵌入常见领域的预构建记忆模板

//go:embed library/*.json
var embeddedLibrary embed.FS

func LoadLibraryMemories(ctx context.Context, store VectorStore, domain string) error {
    // 加载对应领域的预构建记忆
}
```

---

## 7. 实施路线图

### Phase 1：检索精度修复（Week 1-2，预计 5d）

```
[P0] Dual-Content Embedding
  ├── EmbeddingContent() 方法          1d
  ├── 修改 5 个 VectorStore 后端        2d
  ├── 修改 Deduplicator 使用新方法      0.5d
  ├── 修改 AddDraftAndRetrieveSimilar   0.5d
  └── 测试验证                          1d
```

**验收标准**：`WhenToUse` 非空时，向量检索以 `WhenToUse` 为匹配目标，检索结果包含完整 `Content`。

### Phase 2：流程并发化（Week 2-3，预计 7d）

```
[P1] Orchestrator 并发化
  ├── errgroup 并发 Summarize          2d
  ├── Two-Phase Personal Memory        3d
  └── 异步任务管理器增强                2d

[P1] Multi-Query Batch Search
  ├── BatchSearch 方法                 1.5d
  └── 测试                             0.5d
```

**验收标准**：Orchestrator.Summarize 各阶段最大并行执行，总延迟降低 40-60%。

### Phase 3：流水线框架（Week 3-4，预计 10d）

```
[P1] Flow Pipeline 框架
  ├── Step / Pipeline / StepNode 核心   2d
  ├── BuildQuery / MemoryRetrieval      2d
  ├── RerankMemory / RewriteMemory      2d
  ├── MemoryValidation                  1d
  ├── Pipeline 缓存机制                 1d
  └── 测试 + 文档                       2d
```

**验收标准**：可通过 Pipeline API 定义完整记忆流水线，行为与 Python ReMe 等价。

### Phase 4：增强功能（Week 4-5，预计 7d）

```
[P2] Profile 向量后端                   3d
[P2] File Watcher                       2d
[P2] Embedding Cache 持久化             1d
[P2] Tool Memory 参数优化               2d
```

### Phase 5：扩展功能（Week 5-8，预计 14d）

```
[P3] Comparative Extraction             3d
[P3] Memory Utility/Freq + GC           2d
[P3] Registry/Plugin 架构               4d
[P3] MCP Service 模式                   3d
[P3] Benchmark 集成                     2d
[P3] Memory Library                     2d
```

---

## 8. Go 相对 Python 的已有优势

以下功能 agentscope.go **已领先**，无需改变：

| 功能 | Go 实现 | Python 对标 | Go 优势 |
|------|---------|------------|---------|
| **FTS5 + BM25 混合检索** | SQLite FTS5 with CJK-aware tokenization + 两阶段向量召回→BM25重排 | 简单 substring 匹配 | Go 独有，支持 CJK 分词 |
| **RAG 文档解析** | Apache Tika 集成 (`rag/tika.go`) | 无 | Go 独有 |
| **会话快照序列化** | 版本化 JSON snapshot + SaveTo/LoadFrom | 基础序列化 | Go 更规范 |
| **WindowMemory** | 双限制滑动窗口（消息数+token缓冲） | 无独立实现 | Go 独有 |
| **ReMeHook** | HookBeforeModel 拦截模式 | pre_reasoning_hook | Go 更模块化 |
| **并发安全** | sync.Mutex + errgroup | asyncio.Lock | Go 天然优势 |
| **工具结果文件外溢** | ToolResultCompactor + 过期清理 | 类似功能 | Go 保留期更灵活 |
| **Service 层多租户** | service.Storage + RedisStorage | 无独立 service 层 | Go 更完整 |
| **Plan 存储层** | plan.Storage + InMemoryStorage | 无 | Go 独有 |

---

## 9. 附录：关键文件对照表

| 功能 | Python ReMe | agentscope.go |
|------|-------------|---------------|
| MemoryNode 定义 | `reme/core/schema/memory_node.py` | `memory/reme_types.go` |
| Dual-Content Strategy | `memory_node.py:117` (`to_vector_node`) | **缺失** |
| VectorStore 接口 | `reme/core/vector_store/base_vector_store.py` | `memory/vector_store.go` |
| MemoryHandler CRUD | `reme/memory/vector_tools/record/memory_handler.py` | `memory/handler/memory_handler.go` |
| Orchestrator | `reme/memory/vector_based/reme_summarizer.py` | `memory/handler/orchestrator.go` |
| DelegateTask | `reme/memory/vector_tools/delegate_task.py` | **缺失** |
| PersonalSummarizer (两阶段) | `reme/memory/vector_based/personal/personal_summarizer.py` | `memory/summarizer_personal.go` (单阶段) |
| Flow Pipeline | `reme/core/flow/base_flow.py` | **缺失** |
| Batch Search | `memory_handler.py:202` | **缺失** |
| ProfileHandler | `reme/memory/vector_tools/profiles/profile_handler.py` | `memory/handler/profile_handler.go` |
| MemoryValidation | `service.yaml` 流水线步骤 | **缺失** |
| RerankMemory | `service.yaml` 流水线步骤 | **缺失** |
| FileWatcher | `reme/core/file_watcher/` | **缺失** |
| EmbeddingCache | `reme/core/embedding/` | `memory/embedding_cache.go` (无持久化) |
| Benchmark | `benchmark/` 目录 | **缺失** |
| MCP Service | `service.yaml` MCP 配置 | **缺失** |
| Hybrid Search | 简单 substring 匹配 | `memory/hybrid_search.go` + `memory/fts_index.go` (更优) |
| RAG | 无 | `rag/` 包 (Go 独有) |

---

*本文档基于 2026-06-10 的代码状态编写，目标版本: agentscope.go v2.1.0+*
