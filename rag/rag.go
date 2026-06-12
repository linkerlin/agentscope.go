// Package rag 保留轻量 API 以兼容旧引用；新代码请优先使用 memory.LocalVectorStore、memory.ReMeVectorMemory。
package rag

import (
	"context"
	"math"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// Embedder 与 memory.Embedder 一致
type Embedder = memory.Embedder

// VectorStore 简单向量检索存储（预计算向量）
type VectorStore interface {
	Upsert(ctx context.Context, id string, vec []float32, meta map[string]any) error
	Search(ctx context.Context, queryVec []float32, topK int) ([]string, error)
}

// MemoryRAG 将检索结果与对话结合的长期记忆门面
type MemoryRAG interface {
	Retrieve(ctx context.Context, query string, topK int) ([]*message.Msg, error)
	Store(ctx context.Context, id string, text string) error
}

// InMemoryVectorStore 基于内存映射的轻量向量存储
type InMemoryVectorStore struct {
	vecs map[string][]float32
}

// NewInMemoryVectorStore 创建内存向量库
func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		vecs: make(map[string][]float32),
	}
}

// Upsert 插入或更新向量
func (s *InMemoryVectorStore) Upsert(ctx context.Context, id string, vec []float32, meta map[string]any) error {
	s.vecs[id] = make([]float32, len(vec))
	copy(s.vecs[id], vec)
	return nil
}

type scored struct {
	id    string
	score float32
}

// Search 余弦 Top-K 检索
func (s *InMemoryVectorStore) Search(ctx context.Context, queryVec []float32, topK int) ([]string, error) {
	var cands []scored
	for id, v := range s.vecs {
		var dot, n1, n2 float32
		for i := range queryVec {
			dot += queryVec[i] * v[i]
			n1 += queryVec[i] * queryVec[i]
			n2 += v[i] * v[i]
		}
		if n1 == 0 || n2 == 0 {
			continue
		}
		sim := float64(dot) / math.Sqrt(float64(n1)*float64(n2))
		cands = append(cands, scored{id: id, score: float32(sim)})
	}
	sortScores(cands)
	if topK > len(cands) {
		topK = len(cands)
	}
	ids := make([]string, topK)
	for i := 0; i < topK; i++ {
		ids[i] = cands[i].id
	}
	return ids, nil
}

func sortScores(s []scored) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j].score > s[i].score {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
