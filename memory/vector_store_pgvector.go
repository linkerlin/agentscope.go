package memory

import "context"

// PgvectorVectorStore (pilot stub - full in vector/ subpackage for split)
type PgvectorVectorStore struct {
	// TODO
}

// NewPgvectorVectorStore (pilot stub)
func NewPgvectorVectorStore(dsn string, table string, dim int, embed EmbeddingModel) (*PgvectorVectorStore, error) {
	return &PgvectorVectorStore{}, nil
}

// stubs for interface
func (s *PgvectorVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error { return nil }
func (s *PgvectorVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return nil, nil
}
func (s *PgvectorVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	return nil, ErrMemoryNotFound
}
func (s *PgvectorVectorStore) Update(ctx context.Context, node *MemoryNode) error { return nil }
func (s *PgvectorVectorStore) Delete(ctx context.Context, memoryID string) error  { return nil }
func (s *PgvectorVectorStore) DeleteAll(ctx context.Context) error                { return nil }

var _ VectorStore = (*PgvectorVectorStore)(nil)
