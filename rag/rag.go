// Package rag 保留轻量 API 以兼容旧引用；新代码请优先使用 memory.LocalVectorStore、memory.ReMeVectorMemory。
package rag

import (
	"context"

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

// InMemoryVectorStore 为 memory.RawVectorIDStore 的类型别名（内存余弦 Top-K）
type InMemoryVectorStore = memory.RawVectorIDStore

// NewInMemoryVectorStore 创建内存向量库
var NewInMemoryVectorStore = memory.NewRawVectorIDStore
