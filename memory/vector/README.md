# Vector Store Connectors

本目录提供 `VectorStore` 接口的多种实现，用于 ReMe 长期记忆的向量检索后端。

## 已实现后端

| 后端 | 文件 | 状态 |
|------|------|------|
| Local（内存 + 快照） | `vector_store_local.go` | ✅ 完整 |
| Chroma | `vector_store_chroma.go` | ✅ 完整 |
| Qdrant | `vector_store_qdrant.go` | ✅ 完整 |
| Milvus | `vector_store_milvus.go` | ✅ 完整 |
| Elasticsearch | `vector_store_elasticsearch.go` | ⚠️ 占位 |
| Pgvector | `vector_store_pgvector.go` | ⚠️ 占位 |
| Raw（no-op） | `raw_vector_store.go` | ✅ 占位 |

## 快速开始

```go
import (
    "github.com/linkerlin/agentscope.go/embedding"
    "github.com/linkerlin/agentscope.go/memory/vector"
)

embed := embedding.NewOpenAI(apiKey, "text-embedding-3-small")

// Chroma
store, err := vector.NewChromaVectorStore("http://localhost:8000", "my_collection", 1536, embed)

// Qdrant
store, err := vector.NewQdrantVectorStore("http://localhost:6333", "my_collection", 1536, embed)

// Milvus
store, err := vector.NewMilvusVectorStore("http://localhost:19530", "my_collection", 1536, embed)
```

## 集成测试

1. 启动依赖服务：

```bash
cd memory/vector
docker-compose up -d
```

2. 运行集成测试：

```bash
cd memory/vector
VECTOR_STORE_INTEGRATION=1 go test -tags=integration -race -count=1 -timeout=10m
```

3. 停止服务：

```bash
docker-compose down -v
```

## 接口

```go
type VectorStore interface {
    Insert(ctx context.Context, nodes []*MemoryNode) error
    Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
    Get(ctx context.Context, memoryID string) (*MemoryNode, error)
    Update(ctx context.Context, node *MemoryNode) error
    Delete(ctx context.Context, memoryID string) error
    DeleteAll(ctx context.Context) error
}
```

## 注意事项

- 所有远程存储均采用**懒加载**：构造函数不会立即连接，首次操作时才检查/创建集合。
- `DeleteAll` 会删除并重建集合，请谨慎在生产环境使用。
- Milvus 使用 REST API v2，默认端口 `19530`。
- Qdrant 使用 REST API，默认端口 `6333`。
- Chroma 使用 REST API v1，默认端口 `8000`。
