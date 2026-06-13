package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

// WeaviateVectorStore 基于 Weaviate REST API 的远程向量存储实现。
type WeaviateVectorStore = vector.WeaviateVectorStore

// NewWeaviateVectorStore 创建 Weaviate 向量存储。
func NewWeaviateVectorStore(baseURL, className string, dim int, embed EmbeddingModel) (*WeaviateVectorStore, error) {
	return vector.NewWeaviateVectorStore(baseURL, className, dim, embed)
}
