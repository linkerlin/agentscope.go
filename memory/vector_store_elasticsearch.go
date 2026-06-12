package memory

import "github.com/linkerlin/agentscope.go/memory/vector"

type ElasticsearchVectorStore = vector.ElasticsearchVectorStore

func NewElasticsearchVectorStore(addresses []string, index string, dim int, embed EmbeddingModel) (*ElasticsearchVectorStore, error) {
	return vector.NewElasticsearchVectorStore(addresses, index, dim, embed)
}

