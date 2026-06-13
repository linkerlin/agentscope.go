package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// QdrantVectorStore 基于 Qdrant REST API 的远程向量存储实现。
type QdrantVectorStore struct {
	baseURL    string
	collection string
	dim        uint64
	embed      EmbeddingModel
	client     *http.Client
}

// NewQdrantVectorStore 创建 Qdrant 向量存储。
// baseURL 默认 http://localhost:6333
func NewQdrantVectorStore(baseURL, collection string, dim uint64, embed EmbeddingModel) (*QdrantVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if baseURL == "" {
		baseURL = "http://localhost:6333"
	}
	if collection == "" {
		collection = "agentscope_memory"
	}
	s := &QdrantVectorStore{
		baseURL:    baseURL,
		collection: collection,
		dim:        dim,
		embed:      embed,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
	return s, nil
}

func (s *QdrantVectorStore) apiURL(path string) string {
	return fmt.Sprintf("%s/collections/%s%s", s.baseURL, s.collection, path)
}

func (s *QdrantVectorStore) doJSON(ctx context.Context, method, url string, body any) (*http.Response, error) {
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

func (s *QdrantVectorStore) ensureCollection(ctx context.Context) error {
	resp, err := s.doJSON(ctx, http.MethodGet, s.apiURL(""), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body := map[string]any{
		"vectors": map[string]any{
			"size":     s.dim,
			"distance": "Cosine",
		},
	}
	resp, err = s.doJSON(ctx, http.MethodPut, s.apiURL(""), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: failed to create collection, status %d", resp.StatusCode)
	}
	return nil
}

// Insert 插入记忆节点。
func (s *QdrantVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s.embed == nil {
		return ErrEmbeddingRequired
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	type point struct {
		ID      string         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}
	var points []point
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if node.MemoryID == "" {
			node.MemoryID = GenerateMemoryID(node.Content)
		}
		if len(node.Vector) == 0 {
			v, err := s.embed.Embed(ctx, node.EmbeddingContent())
			if err != nil {
				return err
			}
			node.Vector = v
		}
		points = append(points, point{
			ID:     node.MemoryID,
			Vector: node.Vector,
			Payload: map[string]any{
				"content":       node.Content,
				"memory_type":   string(node.MemoryType),
				"memory_target": node.MemoryTarget,
				"when_to_use":   node.WhenToUse,
				"author":        node.Author,
			},
		})
	}
	if len(points) == 0 {
		return nil
	}
	body := map[string]any{"points": points}
	resp, err := s.doJSON(ctx, http.MethodPut, s.apiURL("/points"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: insert failed, status %d", resp.StatusCode)
	}
	return nil
}

// Search 语义检索。
func (s *QdrantVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if s.embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	limit := opts.TopK
	if limit <= 0 {
		limit = 10
	}
	body := map[string]any{
		"vector":       qv,
		"limit":        limit,
		"with_payload": true,
	}
	if opts.MinScore > 0 {
		body["score_threshold"] = opts.MinScore
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/points/search"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: search failed, status %d", resp.StatusCode)
	}
	var result qdrantSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.toNodes(), nil
}

// Get 按 memoryID 读取。
func (s *QdrantVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	body := map[string]any{
		"ids":          []string{memoryID},
		"with_payload": true,
		"with_vector":  false,
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/points"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: get failed, status %d", resp.StatusCode)
	}
	var result qdrantGetResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Result) == 0 {
		return nil, ErrMemoryNotFound
	}
	return result.Result[0].toNode(), nil
}

// Update 覆盖更新。
func (s *QdrantVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	if node == nil || node.MemoryID == "" {
		return ErrInvalidMemoryNode
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	if len(node.Vector) == 0 {
		v, err := s.embed.Embed(ctx, node.EmbeddingContent())
		if err != nil {
			return err
		}
		node.Vector = v
	}
	body := map[string]any{
		"points": []map[string]any{
			{
				"id":     node.MemoryID,
				"vector": node.Vector,
				"payload": map[string]any{
					"content":       node.Content,
					"memory_type":   string(node.MemoryType),
					"memory_target": node.MemoryTarget,
					"when_to_use":   node.WhenToUse,
					"author":        node.Author,
				},
			},
		},
	}
	resp, err := s.doJSON(ctx, http.MethodPut, s.apiURL("/points"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: update failed, status %d", resp.StatusCode)
	}
	return nil
}

// Delete 按 memoryID 删除。
func (s *QdrantVectorStore) Delete(ctx context.Context, memoryID string) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"points": []string{memoryID},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/points/delete"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: delete failed, status %d", resp.StatusCode)
	}
	return nil
}

// DeleteAll 删除集合并重建。
func (s *QdrantVectorStore) DeleteAll(ctx context.Context) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	resp, err := s.doJSON(ctx, http.MethodDelete, s.apiURL(""), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return s.ensureCollection(ctx)
}

type qdrantSearchResult struct {
	Result []qdrantScoredPoint `json:"result"`
}

type qdrantGetResult struct {
	Result []qdrantPoint `json:"result"`
}

type qdrantScoredPoint struct {
	qdrantPoint
	Score float64 `json:"score"`
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Payload map[string]any `json:"payload"`
}

func (p *qdrantPoint) toNode() *MemoryNode {
	return &MemoryNode{
		MemoryID:     p.ID,
		Content:      getPayloadStr(p.Payload, "content"),
		MemoryType:   MemoryType(getPayloadStr(p.Payload, "memory_type")),
		MemoryTarget: getPayloadStr(p.Payload, "memory_target"),
		WhenToUse:    getPayloadStr(p.Payload, "when_to_use"),
		Author:       getPayloadStr(p.Payload, "author"),
	}
}

func (r *qdrantSearchResult) toNodes() []*MemoryNode {
	var nodes []*MemoryNode
	for _, p := range r.Result {
		n := p.toNode()
		n.Score = p.Score
		nodes = append(nodes, n)
	}
	return nodes
}

func getPayloadStr(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

var _ VectorStore = (*QdrantVectorStore)(nil)
