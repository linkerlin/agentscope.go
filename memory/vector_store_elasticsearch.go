package memory

import "context"

// ElasticsearchVectorStore (pilot stub - full in vector/ sub for split)
type ElasticsearchVectorStore struct {
	// TODO: fields in vector/
}

// NewElasticsearchVectorStore (pilot stub)
func NewElasticsearchVectorStore(addresses []string, index string, dim int, embed EmbeddingModel) (*ElasticsearchVectorStore, error) {
	return &ElasticsearchVectorStore{}, nil
}

// Implement VectorStore (stub)
func (s *ElasticsearchVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error { return nil }
func (s *ElasticsearchVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return nil, nil
}
func (s *ElasticsearchVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	return nil, ErrMemoryNotFound
}
func (s *ElasticsearchVectorStore) Update(ctx context.Context, node *MemoryNode) error { return nil }
func (s *ElasticsearchVectorStore) Delete(ctx context.Context, memoryID string) error  { return nil }
func (s *ElasticsearchVectorStore) DeleteAll(ctx context.Context) error                { return nil }

var _ VectorStore = (*ElasticsearchVectorStore)(nil)
