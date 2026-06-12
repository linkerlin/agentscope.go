package memory

import (
	"context"
)

// VectorStore 向量记忆存储接口。
// 实现包括 LocalVectorStore（内存余弦）、Qdrant、Pgvector、Elasticsearch、Chroma、Remote 等。
// 用于 ReMe 向量记忆和 hybrid search。
// 详见 memory/vector/ 轻拆分试点（原审阅报告建议）。
type VectorStore interface {
	Insert(ctx context.Context, nodes []*MemoryNode) error
	Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
	Get(ctx context.Context, memoryID string) (*MemoryNode, error)
	Update(ctx context.Context, node *MemoryNode) error
	Delete(ctx context.Context, memoryID string) error
	DeleteAll(ctx context.Context) error
}
