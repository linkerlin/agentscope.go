package memory

import (
	"context"
	"sort"
)

// RawVectorIDStore 按 ID 存储预计算向量，支持余弦相似度 Top-K（与历史 rag.InMemoryVectorStore 行为一致）
type RawVectorIDStore struct {
	vecs map[string][]float32
	text map[string]string
}

// NewRawVectorIDStore 创建内存向量索引
func NewRawVectorIDStore() *RawVectorIDStore {
	return &RawVectorIDStore{
		vecs: make(map[string][]float32),
		text: make(map[string]string),
	}
}

// Upsert 写入向量；若 meta["text"] 为 string 则一并保存（便于调试）
func (s *RawVectorIDStore) Upsert(ctx context.Context, id string, vec []float32, meta map[string]any) error {
	_ = ctx
	s.vecs[id] = vec
	if t, ok := meta["text"].(string); ok {
		s.text[id] = t
	}
	return nil
}

// Search 按查询向量返回相似度最高的 id 列表
func (s *RawVectorIDStore) Search(ctx context.Context, queryVec []float32, topK int) ([]string, error) {
	_ = ctx
	type scored struct {
		id  string
		sim float64
	}
	var all []scored
	for id, v := range s.vecs {
		all = append(all, scored{id, CosineSimilarity(queryVec, v)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].sim > all[j].sim })
	if topK > len(all) {
		topK = len(all)
	}
	out := make([]string, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, all[i].id)
	}
	return out, nil
}
