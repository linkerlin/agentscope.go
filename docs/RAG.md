# 托管知识库 (RAG Knowledge Base)

AgentScope.Go 的 RAG 托管知识库服务，提供从"文档上传"到"语义检索"的完整管道。
对齐 Python AgentScope 的 `rag/` + `app/rag/` + `middleware/_rag.py`，用 Go 地道方式实现。

## 架构

```
文档上传 → BlobStore → Parser → Chunker → Embedder → VectorStore
                                    ↓
                          KnowledgeBase.Search
                                    ↓
                         RAGMiddleware (自动注入)
```

| 包 | 职责 |
|----|------|
| `rag/document` | Section/Chunk 数据模型（解析与分块的中间载体） |
| `rag/parser` | Parser 接口 + Registry 路由：Text / PDF / PPTX / Image |
| `rag/chunker` | Chunker 接口 + ApproxTokenChunker（近似 token 切分） |
| `rag/blob` | BlobStore 接口 + LocalBlobStore（原始文档字节存储） |
| `rag/kb` | KnowledgeBase 句柄 + KBManager + InMemoryVectorStore（多租户隔离） |
| `rag/index` | Worker 全管道编排 + Queue channel 调度 |
| `middleware` | RAGMiddleware（static 自动检索 / agent 工具 / both） |
| `gateway` | KB HTTP API（CRUD + 上传 + 搜索） |

## 快速使用（库 API）

```go
bs, _ := blob.NewLocalBlobStore("./blobs")
reg := parser.NewRegistry(parser.NewTextParser(), parser.NewPDFParser(), parser.NewPPTXParser())
ch := chunker.NewApproxTokenChunker()
mgr := kb.NewCollectionPerKBStore(kb.NewInMemoryVectorStore(), embedderFactory)

mgr.Create(ctx, kb.Spec{Name: "handbook", EmbedderID: "text-embedding-3-small"})

worker := &index.Worker{Blob: bs, Parsers: reg, Chunker: ch, Manager: mgr}
worker.Process(ctx, index.Task{
    KBName: "handbook", DocID: "doc1", BlobURI: uri,
    MediaType: "application/pdf", Source: "onboarding.pdf",
})

hb, _ := mgr.Get(ctx, "handbook")
results, _ := hb.Search(ctx, "How many PTO days?", 5)
```

## RAG 中间件（自动注入）

将 `RAGMiddleware` 挂到 Agent，让它在每次回复前自动检索知识库并注入上下文：

```go
mw := middleware.NewRAGMiddleware(func(ctx, query string, topK int) ([]middleware.KnowledgeHit, error) {
    results, err := hb.Search(ctx, query, topK)
    // 适配 kb.SearchResult → middleware.KnowledgeHit ...
    return hits, err
}, "handbook")

agent, _ := react.Builder().
    Middleware(mw).  // static + agent 双模式
    Build()
```

- **static 模式**：OnReply 检索 → OnReasoning 注入 HintBlock（agent 无感知）
- **agent 模式**：暴露 `search_knowledge` 工具供 agent 按需调用
- **both 模式**（默认）：两者兼有

## HTTP API

通过 `gateway.NewApp` + `AppConfig.KBService` 一键启用：

```go
svc := gateway.NewKBService(mgr, bs, reg, ch)
srv := gateway.NewApp(gateway.AppConfig{KBService: svc, ...})
srv.RegisterAppRoutes(jwt)  // 自动注册 KB 路由
```

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/knowledge-bases` | 列出知识库 |
| POST | `/api/v1/knowledge-bases` | 创建知识库 |
| GET | `/api/v1/knowledge-bases/{id}` | 知识库详情（含文档列表） |
| DELETE | `/api/v1/knowledge-bases/{id}` | 删除知识库（含集合） |
| POST | `/api/v1/knowledge-bases/{id}/documents` | 上传文档（multipart 或 JSON） |
| GET | `/api/v1/knowledge-bases/{id}/documents` | 列出文档 |
| DELETE | `/api/v1/knowledge-bases/{id}/documents/{doc_id}` | 删除文档 |
| POST | `/api/v1/knowledge-bases/{id}/search` | 语义搜索 |

## 支持的解析器

| 格式 | MIME 类型 | 实现 | 依赖 |
|------|----------|------|------|
| 文本 | text/plain, text/markdown, text/csv, application/json, yaml | TextParser | 纯 stdlib |
| PDF | application/pdf | PDFParser | `ledongthuc/pdf`（纯 Go，无 CGO） |
| PowerPoint | ...presentationml.presentation | PPTXParser | 纯 stdlib（archive/zip + encoding/xml） |
| 图片 | image/png, jpeg, gif, webp | ImageParser | 纯 stdlib（内容嗅探）；OCR 可选 hook |

## 多租户隔离

`KnowledgeBase.Filter` 在 search/list 时始终生效，并强制写入每条记录的 metadata，
确保租户间数据不可互访（defense-in-depth）。

## 可运行示例

```bash
cd examples/rag_kb
go run .
```

无需 API Key，使用确定性 stub embedder 离线运行。生产环境替换为
`embedding.NewOpenAI(...)` / `embedding.NewDashScope(...)`。
