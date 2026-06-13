package memory

import (
	"fmt"

	"github.com/linkerlin/agentscope.go/memory/vector"
)

type QdrantVectorStore = vector.QdrantVectorStore

func NewQdrantVectorStore(host string, port int, collection string, dim uint64, embed EmbeddingModel) (*QdrantVectorStore, error) {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	return vector.NewQdrantVectorStore(baseURL, collection, dim, embed)
}
