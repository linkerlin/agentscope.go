package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type PgvectorVectorStore = vector.PgvectorVectorStore

func NewPgvectorVectorStore(dsn string, table string, dim int, embed EmbeddingModel) (*PgvectorVectorStore, error) {
	return vector.NewPgvectorVectorStore(dsn, table, dim, embed)
}

