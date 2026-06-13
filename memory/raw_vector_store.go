package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type RawVectorStore = vector.RawVectorStore

func NewRawVectorStore(embed EmbeddingModel) *RawVectorStore {
	return vector.NewRawVectorStore(embed)
}
