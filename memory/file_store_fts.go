package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// FTSFileStore 基于 FTS5 + 嵌入模型的 FileStore 实现。
// 对标 ReMe Python SqliteFileStore：将文件分块存于内存并对外暴露混合检索。
type FTSFileStore struct {
	mu      sync.RWMutex
	fts     *FTSIndex
	embed   EmbeddingModel
	chunks  map[string]*FileChunk
	files   map[string]map[string]*FileMetadata
}

// NewFTSFileStore 创建 FTS 文件存储器
func NewFTSFileStore(fts *FTSIndex, embed EmbeddingModel) *FTSFileStore {
	return &FTSFileStore{
		fts:    fts,
		embed:  embed,
		chunks: make(map[string]*FileChunk),
		files:  make(map[string]map[string]*FileMetadata),
	}
}

func (s *FTSFileStore) sourceFiles(source MemorySource) map[string]*FileMetadata {
	m, ok := s.files[string(source)]
	if !ok {
		return make(map[string]*FileMetadata)
	}
	return m
}

func (s *FTSFileStore) UpsertFile(ctx context.Context, meta *FileMetadata, source MemorySource, chunks []*FileChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src := string(source)
	if _, ok := s.files[src]; !ok {
		s.files[src] = make(map[string]*FileMetadata)
	}
	s.files[src][meta.Path] = meta

	for _, ch := range chunks {
		ch.Source = source
		if ch.Hash == "" {
			ch.Hash = hashContent(ch.Text)
		}
		s.chunks[ch.ID] = ch
		if s.fts != nil {
			n := chunkToMemoryNode(ch, source)
			_ = s.fts.Insert(n)
		}
	}
	return nil
}

func (s *FTSFileStore) DeleteFile(ctx context.Context, path string, source MemorySource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src := string(source)
	if m, ok := s.files[src]; ok {
		delete(m, path)
	}
	var toDelete []string
	for id, ch := range s.chunks {
		if ch.Path == path && ch.Source == source {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(s.chunks, id)
		if s.fts != nil {
			_ = s.fts.Delete(id)
		}
	}
	return nil
}

func (s *FTSFileStore) DeleteFileChunks(ctx context.Context, path string, chunkIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	set := make(map[string]bool, len(chunkIDs))
	for _, id := range chunkIDs {
		set[id] = true
	}
	for _, id := range chunkIDs {
		delete(s.chunks, id)
		if s.fts != nil {
			_ = s.fts.Delete(id)
		}
	}
	_ = set
	return nil
}

func (s *FTSFileStore) UpsertChunks(ctx context.Context, chunks []*FileChunk, source MemorySource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range chunks {
		ch.Source = source
		if ch.Hash == "" {
			ch.Hash = hashContent(ch.Text)
		}
		s.chunks[ch.ID] = ch
		if s.fts != nil {
			n := chunkToMemoryNode(ch, source)
			_ = s.fts.Insert(n)
		}
	}
	return nil
}

func (s *FTSFileStore) ListFiles(ctx context.Context, source MemorySource) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := s.sourceFiles(source)
	out := make([]string, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	return out, nil
}

func (s *FTSFileStore) GetFileMetadata(ctx context.Context, path string, source MemorySource) (*FileMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := s.sourceFiles(source)
	meta, ok := m[path]
	if !ok {
		return nil, fmt.Errorf("file_store: file not found: %s", path)
	}
	return meta, nil
}

func (s *FTSFileStore) UpdateFileMetadata(ctx context.Context, meta *FileMetadata, source MemorySource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src := string(source)
	if _, ok := s.files[src]; !ok {
		s.files[src] = make(map[string]*FileMetadata)
	}
	s.files[src][meta.Path] = meta
	return nil
}

func (s *FTSFileStore) GetFileChunks(ctx context.Context, path string, source MemorySource) ([]*FileChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*FileChunk
	for _, ch := range s.chunks {
		if ch.Path == path && ch.Source == source {
			out = append(out, ch)
		}
	}
	return out, nil
}

func (s *FTSFileStore) VectorSearch(ctx context.Context, query string, limit int, sources []MemorySource) ([]*MemorySearchResult, error) {
	if s.embed == nil {
		return nil, fmt.Errorf("file_store: embedding model required for vector search")
	}
	queryVec, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sourceSet := makeSourceSet(sources)
	var results []*MemorySearchResult
	for _, ch := range s.chunks {
		if !sourceSet.Match(ch.Source) {
			continue
		}
		if ch.Embedding == nil {
			continue
		}
		score := CosineSimilarity(queryVec, ch.Embedding)
		results = append(results, &MemorySearchResult{
			Path:      ch.Path,
			StartLine: ch.StartLine,
			EndLine:   ch.EndLine,
			Score:     score,
			Snippet:   truncateSnippet(ch.Text, 200),
			Source:    ch.Source,
			RawMetric: 1.0 - score,
		})
	}
	results = sortByScoreDesc(results)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *FTSFileStore) KeywordSearch(ctx context.Context, query string, limit int, sources []MemorySource) ([]*MemorySearchResult, error) {
	if s.fts == nil {
		return nil, nil
	}
	ftsResults, err := s.fts.Search(query, limit, nil, "")
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sourceSet := makeSourceSet(sources)
	var results []*MemorySearchResult
	for _, r := range ftsResults {
		ch, ok := s.chunks[r.MemoryID]
		if !ok {
			continue
		}
		if !sourceSet.Match(ch.Source) {
			continue
		}
		results = append(results, &MemorySearchResult{
			Path:      ch.Path,
			StartLine: ch.StartLine,
			EndLine:   ch.EndLine,
			Score:     r.BM25Norm,
			Snippet:   truncateSnippet(ch.Text, 200),
			Source:    ch.Source,
			RawMetric: r.BM25Raw,
		})
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *FTSFileStore) HybridSearch(ctx context.Context, query string, limit int, sources []MemorySource, vectorWeight, candidateMultiplier float64) ([]*MemorySearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if vectorWeight <= 0 {
		vectorWeight = 0.7
	}
	if candidateMultiplier <= 0 {
		candidateMultiplier = 3.0
	}
	candidateLimit := int(float64(limit) * candidateMultiplier)
	if candidateLimit > 200 {
		candidateLimit = 200
	}
	if candidateLimit < 1 {
		candidateLimit = 1
	}

	vecResults, vecErr := s.VectorSearch(ctx, query, candidateLimit, sources)
	keyResults, keyErr := s.KeywordSearch(ctx, query, candidateLimit, sources)
	if vecErr != nil && keyErr != nil {
		return nil, fmt.Errorf("file_store: both vector and keyword search failed: %v / %v", vecErr, keyErr)
	}

	merged := mergeHybridResults(vecResults, keyResults, vectorWeight)
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

func (s *FTSFileStore) ClearAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chunks = make(map[string]*FileChunk)
	s.files = make(map[string]map[string]*FileMetadata)
	return nil
}

func (s *FTSFileStore) Close() error {
	if s.fts != nil {
		return s.fts.Close()
	}
	return nil
}

func chunkToMemoryNode(ch *FileChunk, source MemorySource) *MemoryNode {
	n := NewMemoryNode(MemoryTypeSummary, string(source), ch.Text)
	n.MemoryID = ch.ID
	return n
}

func hashContent(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func truncateSnippet(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func sortByScoreDesc(results []*MemorySearchResult) []*MemorySearchResult {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results
}

type sourceSet struct {
	all   bool
	items map[MemorySource]bool
}

func makeSourceSet(sources []MemorySource) sourceSet {
	if len(sources) == 0 {
		return sourceSet{all: true}
	}
	s := sourceSet{items: make(map[MemorySource]bool, len(sources))}
	for _, src := range sources {
		s.items[src] = true
	}
	return s
}

func (s sourceSet) Match(src MemorySource) bool {
	if s.all {
		return true
	}
	return s.items[src]
}

func mergeHybridResults(vecResults, keyResults []*MemorySearchResult, vectorWeight float64) []*MemorySearchResult {
	type entry struct {
		result     *MemorySearchResult
		vecScore   float64
		keyScore   float64
		hasVec     bool
		hasKey     bool
	}
	merged := make(map[string]*entry)

	for _, r := range vecResults {
		k := r.MergeKey()
		merged[k] = &entry{result: r, vecScore: r.Score, hasVec: true}
	}
	for _, r := range keyResults {
		k := r.MergeKey()
		if e, ok := merged[k]; ok {
			e.keyScore = r.Score
			e.hasKey = true
			if r.Score > e.result.Score {
				e.result.Score = r.Score
			}
		} else {
			merged[k] = &entry{result: r, keyScore: r.Score, hasKey: true}
		}
	}

	textWeight := 1.0 - vectorWeight
	var out []*MemorySearchResult
	for _, e := range merged {
		switch {
		case e.hasVec && e.hasKey:
			e.result.Score = e.vecScore*vectorWeight + e.keyScore*textWeight
		case e.hasVec:
			e.result.Score = e.vecScore
		case e.hasKey:
			e.result.Score = e.keyScore
		}
		out = append(out, e.result)
	}
	return sortByScoreDesc(out)
}
