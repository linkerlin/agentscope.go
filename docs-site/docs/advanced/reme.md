# ReMe 长期记忆系统

ReMe（Remember Me）是 AgentScope.Go 的核心差异化能力，提供世界级的长期记忆支持。

## 架构概览

```
┌─────────────────────────────────────────┐
│           MemoryOrchestrator            │
├─────────────┬─────────────┬─────────────┤
│  Personal   │  Procedural │    Tool     │
│  Summarizer │  Summarizer │  Summarizer │
├─────────────┴─────────────┴─────────────┤
│         ReMeVectorMemory                  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐     │
│  │  Vector │ │  FTS5   │ │ Hybrid  │     │
│  │  Store  │ │  Index  │ │ Search  │     │
│  └─────────┘ └─────────┘ └─────────┘     │
└─────────────────────────────────────────┘
```

## 快速使用

### 基础记忆

```go
import "github.com/linkerlin/agentscope.go/memory"

mem := memory.NewInMemoryMemory()
agent, _ := react.Builder().
    Name("Assistant").
    Model(chatModel).
    Memory(mem).
    Build()
```

### 向量记忆

```go
import (
    "github.com/linkerlin/agentscope.go/memory"
    "github.com/linkerlin/agentscope.go/memory/handler"
)

// 创建向量记忆
v, _ := memory.NewReMeVectorMemory(cfg, counter, nil, embedModel)

// 注入编排器，实现自动提取与检索
orch := handler.NewMemoryOrchestrator(
    personalSum, proceduralSum, toolSum,
    memTool, profileTool, historyTool, dedup,
)
v.SetOrchestrator(orch)

// 自动提取记忆
res, _ := v.SummarizeMemory(ctx, msgs, "alice", "coding_task", "")

// 统一检索
nodes, _ := v.RetrieveMemoryUnified(ctx, "Go 最佳实践", "alice", "coding_task", "", 
    memory.RetrieveOptions{TopK: 5})
```

## 向量后端

| 后端 | 包路径 | 说明 |
|------|--------|------|
| Local | `memory/vector_store_local.go` | 本地 SQLite + vec0 |
| Chroma | `memory/vector/vector_store_chroma.go` | ChromaDB REST API |
| Qdrant | `memory/vector/vector_store_qdrant.go` | Qdrant REST API |
| Milvus | `memory/vector/vector_store_milvus.go` | Milvus REST API |
| Pgvector | `memory/vector_store_pgvector.go` | PostgreSQL pgvector |
| Elasticsearch | `memory/vector_store_elasticsearch.go` | ES dense_vector |

## Hybrid Search

结合向量相似度和全文检索（BM25/FTS5）：

```go
results, _ := memory.HybridSearch(ctx, query, vectorResults, ftsResults, 
    memory.HybridOptions{Alpha: 0.7}) // 向量权重 70%
```

## Rerank 精排

二阶段精排提升检索质量：

```go
import "github.com/linkerlin/agentscope.go/rerank"

r := rerank.NewCohereReranker(apiKey)
// 或
r := rerank.NewLocalReranker(embeddingModel)

v.WithReranker(r)
```

## 与 Python 版对比

| 能力 | Python 2.0 | Go 2.0.0 |
|------|-----------|----------|
| 短期记忆 | ✅ 上下文压缩 | ✅ 上下文压缩 |
| 长期记忆 | ❌ 临时移除 | ✅ ReMe 完整实现 |
| 向量后端 | ❌ 无 | ✅ 6 个 |
| Hybrid Search | ❌ 无 | ✅ 向量+全文 |
| Rerank | ❌ 无 | ✅ Cohere/Jina/Local |
| Orchestrator | ❌ 无 | ✅ 自动提取 |
