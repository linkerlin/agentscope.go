package vector

import "context"

// PgvectorVectorStore is a placeholder; full implementation would use pgx/pgvector.
// All operations return ErrNotImplemented.
type PgvectorVectorStore struct{}

// NewPgvectorVectorStore creates a placeholder Pgvector vector store.
func NewPgvectorVectorStore(dsn string, table string, dim int, embed EmbeddingModel) (*PgvectorVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	return &PgvectorVectorStore{}, nil
}

func (s *PgvectorVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	return ErrNotImplemented
}
func (s *PgvectorVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return nil, ErrNotImplemented
}
func (s *PgvectorVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	return nil, ErrNotImplemented
}
func (s *PgvectorVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	return ErrNotImplemented
}
func (s *PgvectorVectorStore) Delete(ctx context.Context, memoryID string) error {
	return ErrNotImplemented
}
func (s *PgvectorVectorStore) DeleteAll(ctx context.Context) error { return ErrNotImplemented }

var _ VectorStore = (*PgvectorVectorStore)(nil)
