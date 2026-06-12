package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type VectorStoreSnapshot = vector.VectorStoreSnapshot

func NewVectorStoreSnapshot(inner VectorStore) *VectorStoreSnapshot {
	return vector.NewVectorStoreSnapshot(inner)
}

