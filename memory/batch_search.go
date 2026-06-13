package memory

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// BatchSearchOptions 批量检索选项
type BatchSearchOptions struct {
	Queries        []string       // 多个查询
	TopK           int            // 每个查询返回数量
	MinScore       float64        // 最低分数阈值
	MemoryTypes    []MemoryType   // 记忆类型过滤
	MemoryTargets  []string       // 记忆目标过滤
	VectorWeight   float64        // 向量权重
	HybridThreshold float64      // 混合阈值（平均相似度过滤）
}

// BatchSearchResult 批量检索结果
type BatchSearchResult struct {
	Query   string
	Nodes   []*MemoryNode
	Score   float64 // 该查询的平均相似度
}

// BatchSearcher 批量检索器，支持多查询并行检索和混合阈值过滤
type BatchSearcher struct {
	store VectorStore
	fts   *FTSIndex
}

// NewBatchSearcher 创建批量检索器
func NewBatchSearcher(store VectorStore, fts *FTSIndex) *BatchSearcher {
	return &BatchSearcher{store: store, fts: fts}
}

// Search 执行批量检索。
// 1. 对每个查询并行执行向量检索
// 2. 可选：混合重排（BM25 + 向量）
// 3. 计算每个结果对所有查询的平均余弦相似度
// 4. 过滤低于 hybridThreshold 的结果
// 5. 合并去重后返回
func (bs *BatchSearcher) Search(ctx context.Context, opts BatchSearchOptions) ([]*MemoryNode, error) {
	if len(opts.Queries) == 0 {
		return nil, nil
	}
	if opts.TopK <= 0 {
		opts.TopK = 5
	}
	if opts.VectorWeight < 0 || opts.VectorWeight > 1 {
		opts.VectorWeight = 0.7
	}

	// 并行执行每个查询的检索
	results := make([]*BatchSearchResult, len(opts.Queries))
	g, ctx := errgroup.WithContext(ctx)

	for i, query := range opts.Queries {
		i, query := i, query
		g.Go(func() error {
			nodes, err := bs.searchSingle(ctx, query, opts)
			if err != nil {
				return err
			}
			results[i] = &BatchSearchResult{
				Query: query,
				Nodes: nodes,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 计算每个结果对所有查询的平均相似度
	return bs.mergeAndFilter(results, opts)
}

// searchSingle 执行单个查询的检索
func (bs *BatchSearcher) searchSingle(ctx context.Context, query string, opts BatchSearchOptions) ([]*MemoryNode, error) {
	// 扩大候选集（3x TopK）用于重排
	retrieveOpts := RetrieveOptions{
		TopK:          opts.TopK * 3,
		MinScore:      opts.MinScore,
		MemoryTypes:   opts.MemoryTypes,
		MemoryTargets: opts.MemoryTargets,
		VectorWeight:  opts.VectorWeight,
	}

	nodes, err := bs.store.Search(ctx, query, retrieveOpts)
	if err != nil {
		return nil, err
	}

	// 混合重排
	if bs.fts != nil && opts.VectorWeight > 0 && opts.VectorWeight < 1 {
		nodes = RankMemoryNodesHybrid(nodes, query, opts.VectorWeight, bs.fts)
	}

	// 截断到 TopK
	if len(nodes) > opts.TopK {
		nodes = nodes[:opts.TopK]
	}
	return nodes, nil
}

// mergeAndFilter 合并结果并过滤
func (bs *BatchSearcher) mergeAndFilter(results []*BatchSearchResult, opts BatchSearchOptions) ([]*MemoryNode, error) {
	// 收集所有节点，计算每个节点在所有查询中的平均相似度
	nodeScores := make(map[string]struct {
		node      *MemoryNode
		scores    []float64
		avgScore  float64
	})

	for _, res := range results {
		for _, n := range res.Nodes {
			if n == nil || n.MemoryID == "" {
				continue
			}
			existing, ok := nodeScores[n.MemoryID]
			if !ok {
				existing.node = n
			}
			existing.scores = append(existing.scores, n.Score)
			nodeScores[n.MemoryID] = existing
		}
	}

	// 计算平均分数并过滤
	var filtered []*MemoryNode
	for mid, data := range nodeScores {
		if len(data.scores) == 0 {
			continue
		}
		// 计算平均相似度
		var sum float64
		for _, s := range data.scores {
			sum += s
		}
		avg := sum / float64(len(data.scores))
		data.avgScore = avg
		nodeScores[mid] = data

		// 如果设置了 hybridThreshold，过滤低质量结果
		if opts.HybridThreshold > 0 && avg < opts.HybridThreshold {
			continue
		}
		data.node.Score = avg
		filtered = append(filtered, data.node)
	}

	// 按平均分数排序
	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[j].Score > filtered[i].Score {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	// 最终截断
	if opts.TopK > 0 && len(filtered) > opts.TopK {
		filtered = filtered[:opts.TopK]
	}
	return filtered, nil
}

// BatchSearchWithEmbedding 使用嵌入模型进行批量检索（先嵌入所有查询，再并行搜索）
type BatchSearchWithEmbedding struct {
	embed  EmbeddingModel
	store  VectorStore
	fts    *FTSIndex
}

// NewBatchSearchWithEmbedding 创建批量嵌入检索器
func NewBatchSearchWithEmbedding(embed EmbeddingModel, store VectorStore, fts *FTSIndex) *BatchSearchWithEmbedding {
	return &BatchSearchWithEmbedding{embed: embed, store: store, fts: fts}
}

// Search 先批量嵌入所有查询，再并行检索
func (be *BatchSearchWithEmbedding) Search(ctx context.Context, opts BatchSearchOptions) ([]*MemoryNode, error) {
	if len(opts.Queries) == 0 {
		return nil, nil
	}

	// 批量嵌入所有查询
	vectors, err := be.embed.EmbedBatch(ctx, opts.Queries)
	if err != nil {
		return nil, err
	}

	// 使用嵌入向量直接搜索（如果 store 支持向量搜索）
	// 否则回退到文本搜索
	var results []*MemoryNode
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)

	for i, query := range opts.Queries {
		i, query := i, query
		g.Go(func() error {
			var nodes []*MemoryNode
			if vecStore, ok := be.store.(VectorSearchByVector); ok && i < len(vectors) {
				// 使用向量直接搜索
				nodes, err = vecStore.SearchByVector(ctx, vectors[i], RetrieveOptions{
					TopK:          opts.TopK,
					MinScore:      opts.MinScore,
					MemoryTypes:   opts.MemoryTypes,
					MemoryTargets: opts.MemoryTargets,
				})
			} else {
				// 回退到文本搜索
				nodes, err = be.store.Search(ctx, query, RetrieveOptions{
					TopK:          opts.TopK,
					MinScore:      opts.MinScore,
					MemoryTypes:   opts.MemoryTypes,
					MemoryTargets: opts.MemoryTargets,
				})
			}
			if err != nil {
				return err
			}
			mu.Lock()
			results = append(results, nodes...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 去重
	seen := make(map[string]bool)
	var unique []*MemoryNode
	for _, n := range results {
		if n == nil || seen[n.MemoryID] {
			continue
		}
		seen[n.MemoryID] = true
		unique = append(unique, n)
	}

	// 排序
	for i := 0; i < len(unique); i++ {
		for j := i + 1; j < len(unique); j++ {
			if unique[j].Score > unique[i].Score {
				unique[i], unique[j] = unique[j], unique[i]
			}
		}
	}

	if opts.TopK > 0 && len(unique) > opts.TopK {
		unique = unique[:opts.TopK]
	}
	return unique, nil
}

// VectorSearchByVector 支持向量直接搜索的接口
type VectorSearchByVector interface {
	SearchByVector(ctx context.Context, vector []float32, opts RetrieveOptions) ([]*MemoryNode, error)
}
