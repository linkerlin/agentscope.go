package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

// QdrantVectorStore is re-exported from the vector subpackage for backward compatibility.
// (Memory light split pilot per review report)
type QdrantVectorStore = vector.QdrantVectorStore

// NewQdrantVectorStore creates a Qdrant-backed vector store and ensures the collection exists.
func NewQdrantVectorStore(host string, port int, collection string, dim uint64, embed EmbeddingModel) (*QdrantVectorStore, error) {
	return vector.NewQdrantVectorStore(host, port, collection, dim, embed)
}
