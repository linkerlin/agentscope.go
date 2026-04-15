package handler

import (
	"context"

	"github.com/linkerlin/agentscope.go/memory"
)

// MemoryHandler 封装向量库的 CRUD 与草稿相似检索
type MemoryHandler struct {
	Store memory.VectorStore
}

// NewMemoryHandler 创建记忆处理器
func NewMemoryHandler(store memory.VectorStore) *MemoryHandler {
	return &MemoryHandler{Store: store}
}

// AddDraftAndRetrieveSimilar 在正式写入前检索相似记忆（用于查重/更新决策）
func (h *MemoryHandler) AddDraftAndRetrieveSimilar(ctx context.Context, node *memory.MemoryNode, topK int) ([]*memory.MemoryNode, error) {
	if h.Store == nil || node == nil {
		return nil, nil
	}
	return h.Store.Search(ctx, node.Content, memory.RetrieveOptions{
		TopK:          topK,
		MinScore:      0.1,
		MemoryTypes:   []memory.MemoryType{node.MemoryType},
		MemoryTargets: []string{node.MemoryTarget},
	})
}

// AddMemory 写入单条记忆
func (h *MemoryHandler) AddMemory(ctx context.Context, node *memory.MemoryNode) error {
	if h.Store == nil {
		return memory.ErrEmbeddingRequired
	}
	return h.Store.Insert(ctx, []*memory.MemoryNode{node})
}

// UpdateMemory 更新记忆
func (h *MemoryHandler) UpdateMemory(ctx context.Context, node *memory.MemoryNode) error {
	if h.Store == nil {
		return memory.ErrEmbeddingRequired
	}
	return h.Store.Update(ctx, node)
}

// DeleteMemory 删除记忆
func (h *MemoryHandler) DeleteMemory(ctx context.Context, memoryID string) error {
	if h.Store == nil {
		return memory.ErrEmbeddingRequired
	}
	return h.Store.Delete(ctx, memoryID)
}

// RetrieveMemory 检索记忆
func (h *MemoryHandler) RetrieveMemory(ctx context.Context, query string, opts memory.RetrieveOptions) ([]*memory.MemoryNode, error) {
	if h.Store == nil {
		return nil, memory.ErrEmbeddingRequired
	}
	return h.Store.Search(ctx, query, opts)
}

// ListMemory 列出记忆（按类型/目标过滤、排序、分页）
func (h *MemoryHandler) ListMemory(ctx context.Context, memType memory.MemoryType, target string, limit int) ([]*memory.MemoryNode, error) {
	if h.Store == nil {
		return nil, memory.ErrEmbeddingRequired
	}
	// VectorStore 接口无原生 List，优先尝试从 LocalVectorStore 全量拉取后过滤
	if lv, ok := h.Store.(*memory.LocalVectorStore); ok {
		return lv.List(memType, target, limit)
	}
	// fallback：用空 query + 过滤器检索
	return h.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          limit,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{memType},
		MemoryTargets: []string{target},
	})
}
