package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type ChromaVectorStore = vector.ChromaVectorStore

func NewChromaVectorStore(baseURL, collection string, dim int, embed EmbeddingModel) (*ChromaVectorStore, error) {
	return vector.NewChromaVectorStore(baseURL, collection, dim, embed)
}
