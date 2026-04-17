package rag

import (
	"context"
	"fmt"

	"github.com/linkerlin/agentscope.go/memory"
	"github.com/linkerlin/agentscope.go/message"
)

// SimpleMemoryRAG 是基于内存向量索引的简单 RAG 实现
type SimpleMemoryRAG struct {
	embedder memory.Embedder
	store    *memory.RawVectorIDStore
	texts    map[string]string
}

// NewSimpleMemoryRAG 创建基于 Embedder 的内存 RAG
func NewSimpleMemoryRAG(embedder memory.Embedder) *SimpleMemoryRAG {
	return &SimpleMemoryRAG{
		embedder: embedder,
		store:    memory.NewRawVectorIDStore(),
		texts:    make(map[string]string),
	}
}

// Store 存储文本（自动生成嵌入）
func (r *SimpleMemoryRAG) Store(ctx context.Context, id string, text string) error {
	vec, err := r.embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	if err := r.store.Upsert(ctx, id, vec, nil); err != nil {
		return err
	}
	r.texts[id] = text
	return nil
}

// Retrieve 根据 query 检索最相关的文本并包装为 message.Msg
func (r *SimpleMemoryRAG) Retrieve(ctx context.Context, query string, topK int) ([]*message.Msg, error) {
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	ids, err := r.store.Search(ctx, vec, topK)
	if err != nil {
		return nil, err
	}
	var msgs []*message.Msg
	for _, id := range ids {
		text, ok := r.texts[id]
		if !ok {
			continue
		}
		msgs = append(msgs, message.NewMsg().TextContent(fmt.Sprintf("[id:%s] %s", id, text)).Build())
	}
	return msgs, nil
}
