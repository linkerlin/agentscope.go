# AgentScope.Go ReMe BM25/FTS5 全文检索实施方案

> 版本：v1.0  
> 目标：用纯 Go 的 `modernc.org/sqlite` + `FTS5` 实现真正的 BM25 全文索引，替代 `memory/hybrid_search.go` 中的词袋重叠率近似。  
> 数据库路径：`{working_dir}/.agentscope/reme.db`

---

## 目录

1. [技术选型](#1-技术选型)
2. [架构设计](#2-架构设计)
3. [数据模型](#3-数据模型)
4. [BM25 与向量融合公式](#4-bm25-与向量融合公式)
5. [接口设计](#5-接口设计)
6. [集成路径](#6-集成路径)
7. [实施步骤](#7-实施步骤)
8. [测试方案](#8-测试方案)

---

## 1. 技术选型

| 候选库 | 优点 | 缺点 | 结论 |
|--------|------|------|------|
| **modernc.org/sqlite** | 纯 Go、无 CGO、默认包含 FTS5、`database/sql` 兼容 | 二进制体积稍大、首次编译慢 | **✅ 选定** |
| github.com/blevesearch/bleve/v2 | 功能全面、分词器插件多 | 引入额外依赖、索引文件独立 | 备选 |
| github.com/blugelabs/bluge | 更现代、性能更好 | 生态较小 | 备选 |
| github.com/saromanov/go-bm25 | 极简零依赖 | 无索引/分词/持久化 | 不适用 |

**选择 `modernc.org/sqlite` 的核心原因**：
1. ReMeLight 的哲学是"Memory as Files"，SQLite 的 `.db` 文件天然契合这一理念。
2. 无需引入 Bleve/ES 等额外索引引擎，一个库解决**持久化存储 + FTS5 全文索引 + BM25 排序**。
3. 纯 Go 实现，完全避免 CGO 在 Windows/容器/WASM 场景下的编译噩梦。

---

## 2. 架构设计

### 2.1 组件关系图

```
┌─────────────────────────────────────────────────────────────────────┐
│                      ReMeVectorMemory                                │
│  ┌─────────────────────┐      ┌─────────────────────────────────┐  │
│  │  LocalVectorStore   │      │         FTSIndex                │  │
│  │  (内存向量检索)      │◄────►│  SQLite + FTS5 + BM25           │  │
│  │  • CosineSimilarity │      │  {working_dir}/.agentscope/     │  │
│  │  • EmbeddingModel   │      │  reme.db                        │  │
│  └─────────────────────┘      └─────────────────────────────────┘  │
│            ▲                              ▲                        │
│            │         HybridSearch         │                        │
│            └──────────────┬───────────────┘                        │
│                           ▼                                        │
│              ┌─────────────────────┐                               │
│              │  RetrieveMemory()   │                               │
│              │  VectorWeight (0,1) │                               │
│              └─────────────────────┘                               │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 设计原则

- **最小侵入**：不改动 `LocalVectorStore` 的内存向量逻辑；`FTSIndex` 作为新增独立组件存在。
- **读写同步**：`FTSIndex` 的 Insert/Update/Delete 与 `LocalVectorStore` 的 CRUD 在 `ReMeVectorMemory` 层同步调用，保证向量与全文索引一致。
- **懒加载**：如果 `FTSIndex` 初始化失败（如磁盘只读），系统回退到纯向量检索，不中断服务。
- **中文友好**：FTS5 表显式指定 `tokenize='unicode61'`，支持 Unicode 边界分词，覆盖中英文混合场景。

---

## 3. 数据模型

### 3.1 FTS5 虚拟表

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
    content,
    memory_target UNINDEXED,
    memory_type UNINDEXED,
    tokenize='unicode61'
);
```

字段说明：
- `content`：被索引的记忆正文（对应 `MemoryNode.Content`）
- `memory_target`：目标用户/任务/工具名，**UNINDEXED**（仅做过滤，不参与全文索引）
- `memory_type`：记忆类型字符串，**UNINDEXED**
- SQLite FTS5 自动隐含 `rowid` 列，我们将它映射为 `MemoryNode.MemoryID`

### 3.2 辅助内容表（可选，用于 FTS5 外部内容模式）

考虑到 `MemoryNode` 本身已完整保存在 `LocalVectorStore`，我们采用 **内部内容模式**（默认模式），即 FTS5 自己维护内容。这样查询时可直接从 `memory_fts` 拿到 `content`，无需 JOIN。

> 如果未来需要减少存储冗余，可迁移到 `contentless` 或 `external content` 模式，届时再评估。

---

## 4. BM25 与向量融合公式

### 4.1 SQLite FTS5 的 `rank`

FTS5 默认 `rank` 列使用 **BM25** 算法，其值特点：
- **越小表示匹配度越高**（对数似然的负数形式）
- 典型范围：`[-100, 0]`

为了与余弦相似度（`[0, 1]`，越大越好）融合，需要统一量纲。

### 4.2 归一化策略

对 BM25 原始值做 **Sigmoid 反转归一化**：

```
bm25Norm = 1 / (1 + exp(rank))   // 当 rank 越负，bm25Norm 越接近 1
```

- 若 `rank = 0`（一般匹配），`bm25Norm ≈ 0.5`
- 若 `rank = -10`（强匹配），`bm25Norm ≈ 0.99995`
- 若 `rank = 10`（弱匹配/正数异常），`bm25Norm ≈ 0.00005`

### 4.3 融合公式（保持现有接口语义）

```go
hybridScore = vectorWeight * vectorSim + (1 - vectorWeight) * bm25Norm
```

其中 `vectorSim` 已经归一化在 `[0, 1]`。

### 4.4 混合检索流程（两阶段）

1. **向量召回**：`LocalVectorStore.Search` 用 `TopK = max(20, opts.TopK*3)` 召回候选，保证召回率。
2. **BM25 精排**：对候选的 `MemoryID` 列表，用 `FTSIndex.BM25Scores(query, ids)` 批量查询 BM25 分，再做融合排序后取最终 `TopK`。

> 为什么不直接用 FTS5 召回？因为 FTS5 无法做语义相似度搜索。两阶段策略兼顾语义与关键词精度，与 Python ReMe 的 `MemorySearch` 设计一致。

---

## 5. 接口设计

### 5.1 FTSIndex

```go
package memory

import "database/sql"

// FTSIndex 封装 SQLite FTS5 全文索引
type FTSIndex struct {
    db *sql.DB
}

// FTSSearchResult FTS 单行搜索结果
type FTSSearchResult struct {
    MemoryID    string
    Content     string
    BM25Raw     float64 // 原始 rank（越小越好）
    BM25Norm    float64 // 归一化到 [0,1]（越大越好）
}

// NewFTSIndex 打开或创建 SQLite 数据库，初始化 FTS5 表
func NewFTSIndex(dbPath string) (*FTSIndex, error)

// Close 关闭数据库连接
func (f *FTSIndex) Close() error

// Insert 插入新记忆到 FTS5
func (f *FTSIndex) Insert(node *MemoryNode) error

// Update 更新已有记忆的 content（FTS5 不支持 UPDATE rowid，需先 Delete 再 Insert）
func (f *FTSIndex) Update(node *MemoryNode) error

// Delete 按 MemoryID 删除
func (f *FTSIndex) Delete(memoryID string) error

// Search 全文检索，返回按 BM25 排序的结果
func (f *FTSIndex) Search(query string, topK int, memType *MemoryType, target string) ([]*FTSSearchResult, error)

// BM25Scores 对指定的 MemoryID 列表批量查询 BM25 分（用于混合重排第二阶段）
func (f *FTSIndex) BM25Scores(query string, memoryIDs []string) (map[string]float64, error)

// Count 返回 FTS5 表中的总记录数（调试用）
func (f *FTSIndex) Count() (int, error)
```

### 5.2 HybridScore 升级

保留 `RankMemoryNodesHybrid` 函数签名，但内部实现改为委托给 `FTSIndex`：

```go
// RankMemoryNodesHybrid 对向量候选做真正的 BM25 融合重排
func RankMemoryNodesHybrid(nodes []*MemoryNode, query string, vectorWeight float64, fts *FTSIndex) []*MemoryNode
```

如果 `fts == nil`，回退到旧版词袋重叠率（保证向后兼容）。

---

## 6. 集成路径

### 6.1 ReMeFileMemory 层

`ReMeFileMemory` 负责初始化 `FTSIndex`，因为它持有 `workingPath`。

```go
// ReMeFileMemory 新增字段
ftx *FTSIndex
```

在 `NewReMeFileMemory` 中：

```go
func NewReMeFileMemory(cfg ReMeFileConfig, counter TokenCounter) (*ReMeFileMemory, error) {
    // ... 现有目录初始化 ...
    dbDir := filepath.Join(base, ".agentscope")
    os.MkdirAll(dbDir, 0o755)
    fts, err := NewFTSIndex(filepath.Join(dbDir, "reme.db"))
    if err != nil {
        // 懒加载：FTS 初始化失败不阻断，仅记录日志
        fts = nil
    }
    m := &ReMeFileMemory{
        // ...
        fts: fts,
    }
    return m, nil
}
```

### 6.2 ReMeVectorMemory 层

`ReMeVectorMemory` 通过内嵌的 `ReMeFileMemory` 访问 `FTSIndex`。

修改 CRUD 方法，同步维护 FTS5：

```go
func (v *ReMeVectorMemory) AddMemory(ctx context.Context, node *MemoryNode) error {
    // 1. 写向量库（原有逻辑）
    err := v.store.Insert(ctx, []*MemoryNode{node})
    if err != nil {
        return err
    }
    // 2. 同步写 FTS5（可选，失败不阻断）
    if v.ReMeFileMemory != nil && v.ReMeFileMemory.fts != nil {
        _ = v.ReMeFileMemory.fts.Insert(node)
    }
    return nil
}
```

同理修改 `UpdateMemory`、`DeleteMemory`。

### 6.3 RetrieveMemory 混合检索入口

```go
func (v *ReMeVectorMemory) RetrieveMemory(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
    if v.store == nil {
        return nil, ErrEmbeddingRequired
    }
    // 1. 向量召回（候选集扩大 3 倍，保证召回率）
    vectorOpts := opts
    if vectorOpts.TopK <= 0 {
        vectorOpts.TopK = 10
    }
    vectorOpts.TopK *= 3
    nodes, err := v.store.Search(ctx, query, vectorOpts)
    if err != nil {
        return nil, err
    }

    w := opts.VectorWeight
    if w > 0 && w < 1 && len(nodes) > 0 && v.fts != nil {
        nodes = RankMemoryNodesHybrid(nodes, query, w, v.fts)
    }

    // 截断到最终 TopK
    if opts.TopK > 0 && len(nodes) > opts.TopK {
        nodes = nodes[:opts.TopK]
    }
    return nodes, nil
}
```

---

## 7. 实施步骤

### Step 1：引入依赖并创建 `fts_index.go`
- `go get modernc.org/sqlite`
- 新建 `memory/fts_index.go`
- 实现 `NewFTSIndex`、`Insert`、`Update`、`Delete`、`Search`、`BM25Scores`
- 编写 `fts_index_test.go`（BM25 基础功能验证）

### Step 2：升级 `hybrid_search.go`
- 修改 `RankMemoryNodesHybrid` 签名，支持传入 `*FTSIndex`
- 内部优先使用 `fts.BM25Scores`；`fts == nil` 时回退旧版 `HybridScore`
- 新增 `bm25Normalize(rank float64) float64` 辅助函数

### Step 3：集成到 `ReMeFileMemory`
- 新增 `fts *FTSIndex` 字段
- 在 `NewReMeFileMemory` 中自动初始化 `{working_dir}/.agentscope/reme.db`
- 提供 `FTSIndex()` 公开方法供外部访问

### Step 4：集成到 `ReMeVectorMemory`
- `AddMemory` / `UpdateMemory` / `DeleteMemory` 同步维护 FTS5
- `RetrieveMemory` 在 `VectorWeight` 混合时启用两阶段检索
- `SaveTo` / `LoadFrom` 考虑是否需要将 `reme.db` 一并复制/恢复（通常 `.db` 文件已持久化，无需额外处理）

### Step 5：验证与示例
- 更新 `examples/reme/vector/main.go` 或 `examples/reme/orchestrator/main.go`，展示 `VectorWeight=0.5` 的混合检索效果
- 确保 `go test ./memory/...` 全绿
- 更新 `TODO.md` 和 `演进方案_ReMe深度整合.md`

---

## 8. 测试方案

| 测试项 | 说明 |
|--------|------|
| `TestFTSIndexCRUD` | Insert/Update/Delete/Count |
| `TestFTSIndexSearchRanking` | 验证 `rank` 与 BM25 排序逻辑（高相关文档排前面） |
| `TestFTSIndexBM25Scores` | 批量查询 BM25 分，验证结果集包含所有请求的 memoryID |
| `TestHybridSearchWithFTS` | `RankMemoryNodesHybrid` 在有 `FTSIndex` 时返回正确融合顺序 |
| `TestHybridSearchFallback` | `FTSIndex=nil` 时回退到旧版词袋重叠率 |
| `TestReMeVectorMemoryFTSIntegration` | `ReMeVectorMemory` 的 Add + Retrieve 端到端，验证 `.agentscope/reme.db` 文件确实生成 |
