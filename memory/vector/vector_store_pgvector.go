package vector

import "context"

// PgvectorVectorStore stub for pilot dedup split.
type PgvectorVectorStore struct{}

// NewPgvectorVectorStore for pilot.
func NewPgvectorVectorStore(dsn string, table string, dim int, embed EmbeddingModel) (*PgvectorVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	return &PgvectorVectorStore{}, nil
}

func (s *PgvectorVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error { return nil }
func (s *PgvectorVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) { return nil, nil }
func (s *PgvectorVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) { return nil, nil }
func (s *PgvectorVectorStore) Update(ctx context.Context, node *MemoryNode) error { return nil }
func (s *PgvectorVectorStore) Delete(ctx context.Context, memoryID string) error { return nil }
func (s *PgvectorVectorStore) DeleteAll(ctx context.Context) error { return nil }

var _ VectorStore = (*PgvectorVectorStore)(nil)



