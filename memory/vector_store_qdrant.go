package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type QdrantVectorStore = vector.QdrantVectorStore

func NewQdrantVectorStore(host string, port int, collection string, dim uint64, embed EmbeddingModel) (*QdrantVectorStore, error) {
	return vector.NewQdrantVectorStore(host, port, collection, dim, embed)
}

