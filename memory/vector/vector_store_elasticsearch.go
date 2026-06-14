package vector

import (
	"context"
	"errors"
)

// ErrNotImplemented indicates a vector store backend is not yet implemented.
var ErrNotImplemented = errors.New("vector store backend not implemented (placeholder)")

// ElasticsearchVectorStore is a placeholder; full implementation would use the Elasticsearch client.
// All operations return ErrNotImplemented.
type ElasticsearchVectorStore struct{}

// NewElasticsearchVectorStore creates a placeholder Elasticsearch vector store.
func NewElasticsearchVectorStore(addresses []string, index string, dim int, embed EmbeddingModel) (*ElasticsearchVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	return &ElasticsearchVectorStore{}, nil
}

func (s *ElasticsearchVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	return ErrNotImplemented
}
func (s *ElasticsearchVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return nil, ErrNotImplemented
}
func (s *ElasticsearchVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	return nil, ErrNotImplemented
}
func (s *ElasticsearchVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	return ErrNotImplemented
}
func (s *ElasticsearchVectorStore) Delete(ctx context.Context, memoryID string) error {
	return ErrNotImplemented
}
func (s *ElasticsearchVectorStore) DeleteAll(ctx context.Context) error { return ErrNotImplemented }

var _ VectorStore = (*ElasticsearchVectorStore)(nil)
