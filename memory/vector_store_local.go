package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type LocalVectorStore = vector.LocalVectorStore

func NewLocalVectorStore(embed EmbeddingModel) *LocalVectorStore {
	return vector.NewLocalVectorStore(embed)
}

var (
	ErrEmbeddingRequired = vector.ErrEmbeddingRequired
	ErrVectorDimension   = vector.ErrVectorDimension
	ErrMemoryNotFound    = vector.ErrMemoryNotFound
	ErrInvalidMemoryNode = vector.ErrInvalidMemoryNode
)
