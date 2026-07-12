package kb

import (
	"context"
	"math"
	"sort"
	"sync"
)

// InMemoryVectorStore is an in-process VectorStore backing one or more
// collections. Suitable for tests and single-process deployments; use a
// remote backend (Qdrant/Milvus/Chroma) for production scale.
type InMemoryVectorStore struct {
	mu          sync.RWMutex
	collections map[string][]Record
}

// NewInMemoryVectorStore creates an empty in-memory store.
func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{collections: make(map[string][]Record)}
}

// Insert adds or replaces records by ID within the collection.
func (s *InMemoryVectorStore) Insert(ctx context.Context, collection string, records []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.collections[collection]
	byID := make(map[string]int, len(existing))
	for i, r := range existing {
		byID[r.ID] = i
	}
	for _, r := range records {
		if idx, ok := byID[r.ID]; ok {
			existing[idx] = r
		} else {
			byID[r.ID] = len(existing)
			existing = append(existing, r)
		}
	}
	s.collections[collection] = existing
	return nil
}

// Search returns the top-K records by cosine similarity, filtered by filter
// (every key/value in filter must match the record's metadata).
func (s *InMemoryVectorStore) Search(ctx context.Context, collection string, queryVec []float32, topK int, filter map[string]any) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []SearchResult
	for _, r := range s.collections[collection] {
		if !matchFilter(r.Metadata, filter) {
			continue
		}
		results = append(results, SearchResult{Record: r, Score: cosine(r.Vector, queryVec)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// DeleteByDoc removes every record whose metadata "doc_id" == docID.
func (s *InMemoryVectorStore) DeleteByDoc(ctx context.Context, collection, docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recs := s.collections[collection]
	kept := recs[:0]
	for _, r := range recs {
		if id, _ := r.Metadata["doc_id"].(string); id != docID {
			kept = append(kept, r)
		}
	}
	s.collections[collection] = kept
	return nil
}

// DeleteCollection drops the collection entirely.
func (s *InMemoryVectorStore) DeleteCollection(ctx context.Context, collection string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.collections, collection)
	return nil
}

// ListDocuments returns distinct doc_ids with first-seen metadata, scoped by filter.
func (s *InMemoryVectorStore) ListDocuments(ctx context.Context, collection string, filter map[string]any) ([]DocInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byDoc := make(map[string]*DocInfo)
	for _, r := range s.collections[collection] {
		if !matchFilter(r.Metadata, filter) {
			continue
		}
		docID, _ := r.Metadata["doc_id"].(string)
		if docID == "" {
			continue
		}
		info, ok := byDoc[docID]
		if !ok {
			source, _ := r.Metadata["source"].(string)
			info = &DocInfo{DocID: docID, Source: source, Metadata: copyMeta(r.Metadata)}
			byDoc[docID] = info
		}
		info.Chunks++
	}
	out := make([]DocInfo, 0, len(byDoc))
	for _, info := range byDoc {
		out = append(out, *info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DocID < out[j].DocID })
	return out, nil
}

func matchFilter(meta, filter map[string]any) bool {
	for k, v := range filter {
		if meta[k] != v {
			return false
		}
	}
	return true
}

func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
