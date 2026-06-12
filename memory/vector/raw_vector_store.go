package vector

import "context"

// RawVectorStore stub for pilot dedup split.
type RawVectorStore struct {
	embed EmbeddingModel
}

// NewRawVectorStore for pilot.
func NewRawVectorStore(embed EmbeddingModel) *RawVectorStore {
	return &RawVectorStore{embed: embed}
}

func (s *RawVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error { return nil }
func (s *RawVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) { return nil, nil }
func (s *RawVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) { return nil, nil }
func (s *RawVectorStore) Update(ctx context.Context, node *MemoryNode) error { return nil }
func (s *RawVectorStore) Delete(ctx context.Context, memoryID string) error { return nil }
func (s *RawVectorStore) DeleteAll(ctx context.Context) error { return nil }

var _ VectorStore = (*RawVectorStore)(nil)

