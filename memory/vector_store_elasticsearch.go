package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ESVectorStore 基于 Elasticsearch 8.x 的远程向量存储实现（使用原生 REST API）
type ESVectorStore struct {
	baseURL string
	index   string
	embed   EmbeddingModel
	dim     int
	client  *http.Client
}

// NewESVectorStore 创建 Elasticsearch 向量存储
func NewESVectorStore(baseURL, index string, dim int, embed EmbeddingModel) (*ESVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if baseURL == "" {
		baseURL = "http://localhost:9200"
	}
	s := &ESVectorStore{
		baseURL: baseURL,
		index:   index,
		embed:   embed,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
	if err := s.ensureIndex(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ESVectorStore) ensureIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fmt.Sprintf("%s/%s", s.baseURL, s.index), nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"content":       map[string]any{"type": "text"},
				"memory_type":   map[string]any{"type": "keyword"},
				"memory_target": map[string]any{"type": "keyword"},
				"when_to_use":   map[string]any{"type": "text"},
				"author":        map[string]any{"type": "keyword"},
				"time_created":  map[string]any{"type": "date"},
				"time_modified": map[string]any{"type": "date"},
				"vector": map[string]any{
					"type":         "dense_vector",
					"dims":         s.dim,
					"similarity":   "cosine",
					"index":        true,
					"element_type": "float",
				},
			},
		},
	}
	data, _ := json.Marshal(mapping)
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/%s", s.baseURL, s.index), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("es: failed to create index, status %d", resp.StatusCode)
	}
	return nil
}

func (s *ESVectorStore) doJSON(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.client.Do(req)
}

// Insert 插入记忆节点
func (s *ESVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s.embed == nil {
		return ErrEmbeddingRequired
	}
	var buf bytes.Buffer
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if node.MemoryID == "" {
			node.MemoryID = GenerateMemoryID(node.Content)
		}
		if len(node.Vector) == 0 {
			v, err := s.embed.Embed(ctx, node.Content)
			if err != nil {
				return err
			}
			node.Vector = v
		}
		meta := map[string]any{"index": map[string]any{"_id": node.MemoryID}}
		_ = json.NewEncoder(&buf).Encode(meta)
		doc := nodeToESDoc(node)
		_ = json.NewEncoder(&buf).Encode(doc)
	}
	if buf.Len() == 0 {
		return nil
	}
	resp, err := s.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/%s/_bulk", s.baseURL, s.index), &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("es: bulk insert failed, status %d", resp.StatusCode)
	}
	return nil
}

// Search 语义检索（使用 ES knn search）
func (s *ESVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if s.embed == nil {
		return nil, ErrEmbeddingRequired
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	k := opts.TopK
	if k <= 0 {
		k = 10
	}
	numCandidates := k * 10
	if numCandidates < 100 {
		numCandidates = 100
	}

	knn := map[string]any{
		"field":          "vector",
		"query_vector":   qv,
		"k":              k,
		"num_candidates": numCandidates,
	}
	filter := buildESFilter(opts)
	if len(filter) > 0 {
		knn["filter"] = filter
	}

	body := map[string]any{
		"knn":    knn,
		"fields": []string{"content", "memory_type", "memory_target", "when_to_use", "author", "time_created", "time_modified"},
		"_source": true,
	}
	resp, err := s.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/%s/_search", s.baseURL, s.index), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("es: search failed, status %d", resp.StatusCode)
	}
	var result esSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.toNodes(opts.MinScore), nil
}

// Get 按 memoryID 读取
func (s *ESVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	resp, err := s.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/%s/_doc/%s", s.baseURL, s.index, memoryID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMemoryNotFound
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("es: get failed, status %d", resp.StatusCode)
	}
	var result esDocResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Found {
		return nil, ErrMemoryNotFound
	}
	n := esSourceToNode(result.ID, result.Source, nil, 0)
	return n, nil
}

// Update 覆盖更新
func (s *ESVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	if node == nil || node.MemoryID == "" {
		return ErrInvalidMemoryNode
	}
	if len(node.Vector) == 0 {
		v, err := s.embed.Embed(ctx, node.Content)
		if err != nil {
			return err
		}
		node.Vector = v
	}
	doc := nodeToESDoc(node)
	resp, err := s.doJSON(ctx, http.MethodPut, fmt.Sprintf("%s/%s/_doc/%s", s.baseURL, s.index, node.MemoryID), doc)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("es: update failed, status %d", resp.StatusCode)
	}
	return nil
}

// Delete 按 memoryID 删除
func (s *ESVectorStore) Delete(ctx context.Context, memoryID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/%s/_doc/%s", s.baseURL, s.index, memoryID), nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("es: delete failed, status %d", resp.StatusCode)
	}
	return nil
}

// DeleteAll 清空索引
func (s *ESVectorStore) DeleteAll(ctx context.Context) error {
	body := map[string]any{"query": map[string]any{"match_all": map[string]any{}}}
	resp, err := s.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/%s/_delete_by_query", s.baseURL, s.index), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("es: delete_all failed, status %d", resp.StatusCode)
	}
	return nil
}

func nodeToESDoc(node *MemoryNode) map[string]any {
	doc := map[string]any{
		"content":       node.Content,
		"memory_type":   string(node.MemoryType),
		"memory_target": node.MemoryTarget,
		"when_to_use":   node.WhenToUse,
		"author":        node.Author,
		"ref_memory_id": node.RefMemoryID,
		"vector":        node.Vector,
	}
	if !node.TimeCreated.IsZero() {
		doc["time_created"] = node.TimeCreated.Format(time.RFC3339)
	}
	if !node.TimeModified.IsZero() {
		doc["time_modified"] = node.TimeModified.Format(time.RFC3339)
	}
	if !node.MessageTime.IsZero() {
		doc["message_time"] = node.MessageTime.Format(time.RFC3339)
	}
	return doc
}

func buildESFilter(opts RetrieveOptions) map[string]any {
	var must []map[string]any
	if len(opts.MemoryTypes) > 0 {
		types := make([]string, len(opts.MemoryTypes))
		for i, t := range opts.MemoryTypes {
			types[i] = string(t)
		}
		must = append(must, map[string]any{"terms": map[string]any{"memory_type": types}})
	}
	if len(opts.MemoryTargets) > 0 {
		if len(opts.MemoryTargets) == 1 {
			must = append(must, map[string]any{"term": map[string]any{"memory_target": opts.MemoryTargets[0]}})
		} else {
			must = append(must, map[string]any{"terms": map[string]any{"memory_target": opts.MemoryTargets}})
		}
	}
	if len(must) == 0 {
		return nil
	}
	return map[string]any{"bool": map[string]any{"must": must}}
}

type esSearchResult struct {
	Hits struct {
		Hits []struct {
			ID     string            `json:"_id"`
			Source map[string]any    `json:"_source"`
			Fields map[string]any    `json:"fields"`
			Score  float64           `json:"_score"`
		} `json:"hits"`
	} `json:"hits"`
}

func (r *esSearchResult) toNodes(minScore float64) []*MemoryNode {
	var nodes []*MemoryNode
	for _, h := range r.Hits.Hits {
		if h.Score < minScore {
			continue
		}
		n := esSourceToNode(h.ID, h.Source, nil, h.Score)
		if n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

type esDocResult struct {
	Found  bool           `json:"found"`
	ID     string         `json:"_id"`
	Source map[string]any `json:"_source"`
}

func esSourceToNode(id string, source map[string]any, vec []float32, score float64) *MemoryNode {
	getStr := func(key string) string {
		if v, ok := source[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	parseTime := func(key string) time.Time {
		s := getStr(key)
		if s == "" {
			return time.Time{}
		}
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	if vec == nil {
		if v, ok := source["vector"]; ok {
			if vf, ok := v.([]any); ok {
				vec = make([]float32, len(vf))
				for i, f := range vf {
					switch val := f.(type) {
					case float64:
						vec[i] = float32(val)
					case float32:
						vec[i] = val
					}
				}
			}
		}
	}
	return &MemoryNode{
		MemoryID:     id,
		Content:      getStr("content"),
		MemoryType:   MemoryType(getStr("memory_type")),
		MemoryTarget: getStr("memory_target"),
		WhenToUse:    getStr("when_to_use"),
		Author:       getStr("author"),
		RefMemoryID:  getStr("ref_memory_id"),
		TimeCreated:  parseTime("time_created"),
		TimeModified: parseTime("time_modified"),
		MessageTime:  parseTime("message_time"),
		Vector:       vec,
		Score:        score,
	}
}

var _ VectorStore = (*ESVectorStore)(nil)
