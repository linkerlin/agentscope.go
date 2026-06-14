package memory

import (
	"context"
	"fmt"
	"sort"
)

// CrossModalSearchResult 跨模态检索结果
type CrossModalSearchResult struct {
	MemoryID    string                `json:"memory_id"`
	ContentType MultimodalContentType `json:"content_type"`
	Score       float64               `json:"score"`
	Content     string                `json:"content"`
	URL         string                `json:"url,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty"`
}

// CrossModalSearcher 跨模态检索器
type CrossModalSearcher struct {
	store         VectorStore
	imageEmbedder ImageEmbeddingModel
	audioEmbedder AudioEmbeddingModel
	textEmbedder  EmbeddingModel
}

// NewCrossModalSearcher 创建跨模态检索器
func NewCrossModalSearcher(store VectorStore, textEmbedder EmbeddingModel, imageEmbedder ImageEmbeddingModel, audioEmbedder AudioEmbeddingModel) *CrossModalSearcher {
	return &CrossModalSearcher{
		store:         store,
		textEmbedder:  textEmbedder,
		imageEmbedder: imageEmbedder,
		audioEmbedder: audioEmbedder,
	}
}

// SearchByText 文本查询 → 跨模态结果（文本 + 图像 + 音频）
func (cms *CrossModalSearcher) SearchByText(ctx context.Context, query string, topK int) ([]*CrossModalSearchResult, error) {
	if cms.store == nil || cms.textEmbedder == nil {
		return nil, fmt.Errorf("store or text embedder not configured")
	}

	// 1. 文本嵌入
	_, err := cms.textEmbedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("text embed failed: %w", err)
	}

	// 2. 向量检索（获取所有类型结果）
	nodes, err := cms.store.Search(ctx, query, RetrieveOptions{
		TopK:     topK * 3, // 扩大候选集
		MinScore: 0.0,
	})
	if err != nil {
		return nil, err
	}

	// 3. 分类整理结果
	var results []*CrossModalSearchResult
	for _, node := range nodes {
		if node == nil {
			continue
		}

		// 判断内容类型（通过 metadata 或内容特征）
		contentType := ContentTypeText
		url := ""
		if node.Metadata != nil {
			if ct, ok := node.Metadata["content_type"].(string); ok {
				contentType = MultimodalContentType(ct)
			}
			if u, ok := node.Metadata["url"].(string); ok {
				url = u
			}
		}

		result := &CrossModalSearchResult{
			MemoryID:    node.MemoryID,
			ContentType: contentType,
			Score:       node.Score,
			Content:     node.Content,
			URL:         url,
			Metadata:    node.Metadata,
		}
		results = append(results, result)
	}

	// 4. 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 5. 截断到 topK
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// SearchByImage 图像查询 → 相似图像/文本
func (cms *CrossModalSearcher) SearchByImage(ctx context.Context, imageURL string, base64 string, topK int) ([]*CrossModalSearchResult, error) {
	if cms.store == nil || cms.imageEmbedder == nil {
		return nil, fmt.Errorf("store or image embedder not configured")
	}

	// 1. 图像嵌入
	imageVec, err := cms.imageEmbedder.EmbedImage(ctx, imageURL, base64)
	if err != nil {
		return nil, fmt.Errorf("image embed failed: %w", err)
	}

	// 2. 使用向量直接搜索（如果 store 支持）
	var nodes []*MemoryNode
	if vecStore, ok := cms.store.(VectorSearchByVector); ok {
		nodes, err = vecStore.SearchByVector(ctx, imageVec, RetrieveOptions{
			TopK:     topK * 2,
			MinScore: 0.0,
		})
	} else {
		// 回退：使用图像文件名作为文本查询
		query := fmt.Sprintf("image similar to %s", imageURL)
		if imageURL == "" {
			query = "image content"
		}
		nodes, err = cms.store.Search(ctx, query, RetrieveOptions{
			TopK:     topK * 2,
			MinScore: 0.0,
		})
	}
	if err != nil {
		return nil, err
	}

	// 3. 整理结果
	var results []*CrossModalSearchResult
	for _, node := range nodes {
		if node == nil {
			continue
		}
		contentType := ContentTypeText
		if node.Metadata != nil {
			if ct, ok := node.Metadata["content_type"].(string); ok {
				contentType = MultimodalContentType(ct)
			}
		}
		results = append(results, &CrossModalSearchResult{
			MemoryID:    node.MemoryID,
			ContentType: contentType,
			Score:       node.Score,
			Content:     node.Content,
			Metadata:    node.Metadata,
		})
	}

	// 4. 排序并截断
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// SearchByAudio 音频查询 → 相似音频/文本
func (cms *CrossModalSearcher) SearchByAudio(ctx context.Context, audioURL string, base64 string, topK int) ([]*CrossModalSearchResult, error) {
	if cms.store == nil || cms.audioEmbedder == nil {
		return nil, fmt.Errorf("store or audio embedder not configured")
	}

	// 1. 音频嵌入
	audioVec, err := cms.audioEmbedder.EmbedAudio(ctx, audioURL, base64)
	if err != nil {
		return nil, fmt.Errorf("audio embed failed: %w", err)
	}

	// 2. 使用向量直接搜索
	var nodes []*MemoryNode
	if vecStore, ok := cms.store.(VectorSearchByVector); ok {
		nodes, err = vecStore.SearchByVector(ctx, audioVec, RetrieveOptions{
			TopK:     topK * 2,
			MinScore: 0.0,
		})
	} else {
		query := fmt.Sprintf("audio similar to %s", audioURL)
		if audioURL == "" {
			query = "audio content"
		}
		nodes, err = cms.store.Search(ctx, query, RetrieveOptions{
			TopK:     topK * 2,
			MinScore: 0.0,
		})
	}
	if err != nil {
		return nil, err
	}

	// 3. 整理结果
	var results []*CrossModalSearchResult
	for _, node := range nodes {
		if node == nil {
			continue
		}
		contentType := ContentTypeText
		if node.Metadata != nil {
			if ct, ok := node.Metadata["content_type"].(string); ok {
				contentType = MultimodalContentType(ct)
			}
		}
		results = append(results, &CrossModalSearchResult{
			MemoryID:    node.MemoryID,
			ContentType: contentType,
			Score:       node.Score,
			Content:     node.Content,
			Metadata:    node.Metadata,
		})
	}

	// 4. 排序并截断
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// MultimodalReranker 多模态重排器
type MultimodalReranker struct {
	// 可配置不同模态的权重
	TextWeight  float64
	ImageWeight float64
	AudioWeight float64
}

// NewMultimodalReranker 创建多模态重排器
func NewMultimodalReranker() *MultimodalReranker {
	return &MultimodalReranker{
		TextWeight:  1.0,
		ImageWeight: 0.8,
		AudioWeight: 0.7,
	}
}

// Rerank 对跨模态结果进行重排
func (mr *MultimodalReranker) Rerank(results []*CrossModalSearchResult) []*CrossModalSearchResult {
	// 根据模态类型调整分数
	for _, r := range results {
		switch r.ContentType {
		case ContentTypeText:
			r.Score *= mr.TextWeight
		case ContentTypeImage:
			r.Score *= mr.ImageWeight
		case ContentTypeAudio:
			r.Score *= mr.AudioWeight
		case ContentTypeVideo:
			// 视频综合文本+图像权重
			r.Score *= (mr.TextWeight + mr.ImageWeight) / 2
		}
	}

	// 重新排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// CrossModalMemoryStore 跨模态记忆存储接口
type CrossModalMemoryStore interface {
	VectorStore
	// StoreMultimodal 存储多模态记忆
	StoreMultimodal(ctx context.Context, node *MultimodalMemoryNode) error
	// SearchCrossModal 跨模态检索
	SearchCrossModal(ctx context.Context, query string, queryType MultimodalContentType, topK int) ([]*CrossModalSearchResult, error)
}
