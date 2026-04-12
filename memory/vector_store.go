package memory

import (
	"context"
)

// VectorStore 向量记忆存储（首版以内存实现为主，可扩展远程后端）
type VectorStore interface {
	Insert(ctx context.Context, nodes []*MemoryNode) error
	Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error)
	Get(ctx context.Context, memoryID string) (*MemoryNode, error)
	Update(ctx context.Context, node *MemoryNode) error
	Delete(ctx context.Context, memoryID string) error
	DeleteAll(ctx context.Context) error
}
