# rag_kb — RAG 托管知识库管道示例

演示 AgentScope.Go 的完整 RAG 索引管道：**blob → parse → chunk → embed → insert → search**。

## 运行

```bash
cd examples/rag_kb
go run .
```

无需 API Key —— 使用确定性 stub embedder 离线运行。生产环境将其替换为
`embedding.NewOpenAI(...)` / `embedding.NewDashScope(...)` 即可。

## 管道组成

| 阶段 | 包 | 说明 |
|------|----|------|
| Blob 存储 | `rag/blob` | `LocalBlobStore` 存原始文档字节 |
| 解析 | `rag/parser` | `Registry` 按 MIME 路由：Text/PDF/PPTX/Image |
| 分块 | `rag/chunker` | `ApproxTokenChunker` 近似 token 切分 |
| 知识库 | `rag/kb` | `KBManager`(一 KB 一 collection) + `KnowledgeBase` 句柄 |
| 索引 | `rag/index` | `Worker` 串联整条管道 + `Queue` channel 调度 |

## 添加 PDF/PPTX 支持

```go
reg := parser.NewRegistry(
    parser.NewTextParser(),
    parser.NewPDFParser(),    // 需 go get github.com/ledongthuc/pdf
    parser.NewPPTXParser(),   // 纯 stdlib (archive/zip + encoding/xml)
    parser.NewImageParser(),  // 内容嗅探 → DataBlock，可选 OCR hook
)
```
