package handler

import (
	"context"

	"github.com/linkerlin/agentscope.go/memory"
)

// MemoryHandler 封装向量库的 CRUD 与草稿相似检索
type MemoryHandler struct {
	Store memory.VectorStore
}

// BatchSearchQuery 单次批量检索中的子查询
type BatchSearchQuery struct {
	Query    string
	TopK     int
	MinScore float64
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
	return h.Store.Search(ctx, node.EmbeddingContent(), memory.RetrieveOptions{
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

// BatchSearch 跨多查询批量检索并去重。
// 若 hybridThreshold > 0 且 store 为 LocalVectorStore，则按各结果对全部查询的平均余弦相似度过滤。
func (h *MemoryHandler) BatchSearch(ctx context.Context, queries []BatchSearchQuery, hybridThreshold float64) ([]*memory.MemoryNode, error) {
	if h.Store == nil || len(queries) == 0 {
		return nil, nil
	}

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
		for _, n := range seen {
			result = append(result, n)
		}
		return result, nil
	}

	lv, ok := h.Store.(*memory.LocalVectorStore)
	if !ok {
		result := make([]*memory.MemoryNode, 0, len(seen))
		for _, n := range seen {
			result = append(result, n)
		}
		return result, nil
	}

	queryStrings := make([]string, len(queries))
	for i, q := range queries {
		queryStrings[i] = q.Query
	}
	filtered := lv.BatchSearchWithThreshold(queryStrings, seen, hybridThreshold)
	return filtered, nil
}

// ListMemory 列出记忆（按类型/目标过滤、排序、分页）
func (h *MemoryHandler) ListMemory(ctx context.Context, memType memory.MemoryType, target string, limit int) ([]*memory.MemoryNode, error) {
	if h.Store == nil {
		return nil, memory.ErrEmbeddingRequired
	}
	if lv, ok := h.Store.(*memory.LocalVectorStore); ok {
		return lv.List(memType, target, limit)
	}
	return h.Store.Search(ctx, "", memory.RetrieveOptions{
		TopK:          limit,
		MinScore:      0,
		MemoryTypes:   []memory.MemoryType{memType},
		MemoryTargets: []string{target},
	})
}
