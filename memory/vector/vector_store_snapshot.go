package vector

import "context"

// VectorStoreSnapshot stub for pilot dedup split.
type VectorStoreSnapshot struct {
	inner VectorStore
}

// NewVectorStoreSnapshot for pilot.
func NewVectorStoreSnapshot(inner VectorStore) *VectorStoreSnapshot {
	return &VectorStoreSnapshot{inner: inner}
}

func (s *VectorStoreSnapshot) Insert(ctx context.Context, nodes []*MemoryNode) error {
	return s.inner.Insert(ctx, nodes)
}
func (s *VectorStoreSnapshot) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	return s.inner.Search(ctx, query, opts)
}
func (s *VectorStoreSnapshot) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	return s.inner.Get(ctx, memoryID)
}
func (s *VectorStoreSnapshot) Update(ctx context.Context, node *MemoryNode) error {
	return s.inner.Update(ctx, node)
}
func (s *VectorStoreSnapshot) Delete(ctx context.Context, memoryID string) error {
	return s.inner.Delete(ctx, memoryID)
}
func (s *VectorStoreSnapshot) DeleteAll(ctx context.Context) error {
	return s.inner.DeleteAll(ctx)
}

var _ VectorStore = (*VectorStoreSnapshot)(nil)
