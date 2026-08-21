// Package kb provides the runtime knowledge-base handle and its manager.
// A KnowledgeBase binds an embedding model with a vector-store collection
// (optionally scoped by a metadata filter for multi-tenancy) and exposes
// search/insert/delete/list operations.
//
// Aligned with Python AgentScope rag.KnowledgeBase + app.rag.knowledge_base_manager.
package kb

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkerlin/agentscope.go/rag/document"
)

// Embedder computes a vector for text. Mirrors vector.EmbeddingModel but kept
// local to avoid a hard dependency on the memory package.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Record is one indexed chunk stored in a collection.
type Record struct {
	ID       string         `json:"id"`
	Vector   []float32      `json:"vector,omitempty"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata"`
}

// SearchResult is a hit from a vector search.
type SearchResult struct {
	Record
	Score float64 `json:"score"`
}

// VectorStore is the storage backend for knowledge-base collections. It takes
// pre-computed vectors — the KnowledgeBase binds its own Embedder and computes
// vectors before delegating to the store. A metadata filter scopes search and
// list to a tenant/subset.
type VectorStore interface {
	// Insert adds or replaces records in the named collection (created lazily).
	Insert(ctx context.Context, collection string, records []Record) error
	// Search returns the top-K records nearest to queryVec, filtered by filter.
	Search(ctx context.Context, collection string, queryVec []float32, topK int, filter map[string]any) ([]SearchResult, error)
	// DeleteByDoc removes every record whose metadata "doc_id" matches docID.
	DeleteByDoc(ctx context.Context, collection, docID string) error
	// DeleteCollection drops the entire collection.
	DeleteCollection(ctx context.Context, collection string) error
	// ListDocuments returns the distinct doc_ids (with first-seen metadata) in
	// the collection, scoped by filter.
	ListDocuments(ctx context.Context, collection string, filter map[string]any) ([]DocInfo, error)
	// ListChunks returns every record of one document (metadata "doc_id" ==
	// docID), scoped by filter, ordered by metadata "chunk_index".
	ListChunks(ctx context.Context, collection, docID string, filter map[string]any) ([]Record, error)
}

// DocInfo summarises one indexed document in a collection.
type DocInfo struct {
	DocID    string         `json:"doc_id"`
	Source   string         `json:"source"`
	Chunks   int            `json:"chunks"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// KnowledgeBase is the runtime handle for one knowledge base: it binds an
// embedding model and a (collection, filter) scope. Cheap to construct; the
// collection is created lazily on first insert/search.
type KnowledgeBase struct {
	Name        string
	Description string
	Embedder    Embedder
	Store       VectorStore
	Collection  string
	// Filter is always applied to search/list and forced onto every inserted
	// record's metadata (defense-in-depth multi-tenant scoping).
	Filter map[string]any
}

// Search embeds the query and retrieves the top-K chunks.
func (kb *KnowledgeBase) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}
	vec, err := kb.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("kb: embed query: %w", err)
	}
	return kb.Store.Search(ctx, kb.Collection, vec, topK, kb.Filter)
}

// InsertChunks embeds and inserts the given chunks as records of one document.
// docID is forced onto every record's metadata; the KB filter is also applied.
func (kb *KnowledgeBase) InsertChunks(ctx context.Context, docID string, chunks []document.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	records := make([]Record, 0, len(chunks))
	for _, c := range chunks {
		text := c.Text()
		if text == "" {
			continue // DataBlock-only chunks are skipped for text embedding
		}
		vec, err := kb.Embedder.Embed(ctx, text)
		if err != nil {
			return fmt.Errorf("kb: embed chunk %d: %w", c.ChunkIndex, err)
		}
		meta := mergeMeta(c.Metadata, kb.Filter)
		meta["doc_id"] = docID
		meta["source"] = c.Source
		meta["chunk_index"] = c.ChunkIndex
		meta["total_chunks"] = c.TotalChunks
		records = append(records, Record{
			ID:       fmt.Sprintf("%s_%d", docID, c.ChunkIndex),
			Vector:   vec,
			Text:     text,
			Metadata: meta,
		})
	}
	if len(records) == 0 {
		return nil
	}
	return kb.Store.Insert(ctx, kb.Collection, records)
}

// DeleteDocument removes all chunks of one document.
func (kb *KnowledgeBase) DeleteDocument(ctx context.Context, docID string) error {
	return kb.Store.DeleteByDoc(ctx, kb.Collection, docID)
}

// ListDocuments returns the documents indexed in this knowledge base.
func (kb *KnowledgeBase) ListDocuments(ctx context.Context) ([]DocInfo, error) {
	return kb.Store.ListDocuments(ctx, kb.Collection, kb.Filter)
}

// ListChunks returns every indexed chunk of one document, ordered by chunk
// index. Vector payloads are not included.
func (kb *KnowledgeBase) ListChunks(ctx context.Context, docID string) ([]Record, error) {
	recs, err := kb.Store.ListChunks(ctx, kb.Collection, docID, kb.Filter)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(recs))
	for _, r := range recs {
		r.Vector = nil
		out = append(out, r)
	}
	return out, nil
}

func mergeMeta(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// Spec is the persisted description of a knowledge base (name, collection,
// embedding model id, filter). Stored by the KBManager.
type Spec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Collection  string         `json:"collection"`
	EmbedderID  string         `json:"embedder_id"`
	Filter      map[string]any `json:"filter,omitempty"`
}

// EmbedderFactory resolves an embedder by the id stored in a Spec.
type EmbedderFactory func(embedderID string) (Embedder, error)

// KBManager owns knowledge-base lifecycle and hands out runtime handles.
// CollectionPerKB allocates one collection per knowledge base, so any
// embedding dimension is allowed.
type KBManager struct {
	mu         sync.RWMutex
	specs      map[string]Spec
	store      VectorStore
	embedders  EmbedderFactory
	collection func(name string) string
}

// NewCollectionPerKBManager builds a manager using the given vector store and
// embedder factory. Each KB gets its own collection named "kb_<name>".
func NewCollectionPerKBManager(store VectorStore, embedders EmbedderFactory) *KBManager {
	return &KBManager{
		specs:      make(map[string]Spec),
		store:      store,
		embedders:  embedders,
		collection: func(name string) string { return "kb_" + name },
	}
}

// Create registers a new knowledge base. Returns an error if the name exists.
func (m *KBManager) Create(ctx context.Context, spec Spec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.specs[spec.Name]; exists {
		return fmt.Errorf("kb: knowledge base %q already exists", spec.Name)
	}
	if spec.Collection == "" {
		spec.Collection = m.collection(spec.Name)
	}
	m.specs[spec.Name] = spec
	return nil
}

// Get returns the runtime handle for a knowledge base.
func (m *KBManager) Get(ctx context.Context, name string) (*KnowledgeBase, error) {
	m.mu.RLock()
	spec, ok := m.specs[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("kb: knowledge base %q not found", name)
	}
	emb, err := m.embedders(spec.EmbedderID)
	if err != nil {
		return nil, fmt.Errorf("kb: resolve embedder %q: %w", spec.EmbedderID, err)
	}
	return &KnowledgeBase{
		Name:        spec.Name,
		Description: spec.Description,
		Embedder:    emb,
		Store:       m.store,
		Collection:  spec.Collection,
		Filter:      spec.Filter,
	}, nil
}

// List returns all registered knowledge-base specs.
func (m *KBManager) List(ctx context.Context) []Spec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Spec, 0, len(m.specs))
	for _, s := range m.specs {
		out = append(out, s)
	}
	return out
}

// Delete removes a knowledge base and drops its collection.
func (m *KBManager) Delete(ctx context.Context, name string) error {
	m.mu.Lock()
	spec, ok := m.specs[name]
	if ok {
		delete(m.specs, name)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("kb: knowledge base %q not found", name)
	}
	return m.store.DeleteCollection(ctx, spec.Collection)
}
