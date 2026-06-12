package rag

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// SimpleMemoryRAG 是基于内存向量索引的简单 RAG 实现
type SimpleMemoryRAG struct {
	embedder memory.Embedder
	vecs     map[string][]float32
	texts    map[string]string
}

// NewSimpleMemoryRAG 创建基于 Embedder 的内存 RAG
func NewSimpleMemoryRAG(embedder memory.Embedder) *SimpleMemoryRAG {
	return &SimpleMemoryRAG{
		embedder: embedder,
		vecs:     make(map[string][]float32),
		texts:    make(map[string]string),
	}
}

// Store 存储文本（自动生成嵌入）
func (r *SimpleMemoryRAG) Store(ctx context.Context, id string, text string) error {
	vec, err := r.embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	r.vecs[id] = vec
	r.texts[id] = text
	return nil
}

// cosineSim 计算两个向量的余弦相似度
func cosineSim(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// Retrieve 根据 query 检索最相关的文本并包装为 message.Msg
func (r *SimpleMemoryRAG) Retrieve(ctx context.Context, query string, topK int) ([]*message.Msg, error) {
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	type scored struct {
		id    string
		score float32
	}
	var candidates []scored
	for id, v := range r.vecs {
		candidates = append(candidates, scored{id: id, score: cosineSim(vec, v)})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}
	var msgs []*message.Msg
	for i := 0; i < topK; i++ {
		id := candidates[i].id
		msgs = append(msgs, message.NewMsg().TextContent(fmt.Sprintf("[id:%s] %s", id, r.texts[id])).Build())
	}
	return msgs, nil
}
