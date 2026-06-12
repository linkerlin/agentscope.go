package vector

import "context"

// QdrantVectorStore stub for pilot dedup split.
type QdrantVectorStore struct{}

// NewQdrantVectorStore for pilot.
func NewQdrantVectorStore(host string, port int, collection string, dim uint64, embed EmbeddingModel) (*QdrantVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	return &QdrantVectorStore{}, nil
}

func (s *QdrantVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error { return nil }
func (s *QdrantVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) { return nil, nil }
func (s *QdrantVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) { return nil, nil }
func (s *QdrantVectorStore) Update(ctx context.Context, node *MemoryNode) error { return nil }
func (s *QdrantVectorStore) Delete(ctx context.Context, memoryID string) error { return nil }
func (s *QdrantVectorStore) DeleteAll(ctx context.Context) error { return nil }

var _ VectorStore = (*QdrantVectorStore)(nil)



