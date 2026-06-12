package vector

import "context"

// ElasticsearchVectorStore is a stub for pilot; full impl would use elasticsearch client.
// For complete dedup split, this is the location for the impl.
type ElasticsearchVectorStore struct {
	// TODO: fields, client etc.
}

// NewElasticsearchVectorStore for pilot.
func NewElasticsearchVectorStore(addresses []string, index string, dim int, embed EmbeddingModel) (*ElasticsearchVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	return &ElasticsearchVectorStore{}, nil
}

// Implement VectorStore
func (s *ElasticsearchVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error { return nil }
func (s *ElasticsearchVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) { return nil, nil }
func (s *ElasticsearchVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) { return nil, nil }
func (s *ElasticsearchVectorStore) Update(ctx context.Context, node *MemoryNode) error { return nil }
func (s *ElasticsearchVectorStore) Delete(ctx context.Context, memoryID string) error { return nil }
func (s *ElasticsearchVectorStore) DeleteAll(ctx context.Context) error { return nil }

var _ VectorStore = (*ElasticsearchVectorStore)(nil)



