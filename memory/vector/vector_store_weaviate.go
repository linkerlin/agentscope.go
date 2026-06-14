package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WeaviateVectorStore 基于 Weaviate REST API (v1) 的远程向量存储实现。
type WeaviateVectorStore struct {
	baseURL   string
	className string
	dim       int
	embed     EmbeddingModel
	client    *http.Client
}

// NewWeaviateVectorStore 创建 Weaviate 向量存储。
// baseURL 默认 http://localhost:8080
func NewWeaviateVectorStore(baseURL, className string, dim int, embed EmbeddingModel) (*WeaviateVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if className == "" {
		className = "AgentScopeMemory"
	}
	s := &WeaviateVectorStore{
		baseURL:   baseURL,
		className: className,
		dim:       dim,
		embed:     embed,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
	return s, nil
}

func (s *WeaviateVectorStore) apiURL(path string) string {
	return fmt.Sprintf("%s/v1%s", s.baseURL, path)
}

func (s *WeaviateVectorStore) doJSON(ctx context.Context, method, url string, body any) (*http.Response, error) {
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
	req.Header.Set("Accept", "application/json")
	return s.client.Do(req)
}

func (s *WeaviateVectorStore) ensureSchema(ctx context.Context) error {
	// Check if class exists
	resp, err := s.doJSON(ctx, http.MethodGet, s.apiURL("/schema/"+s.className), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Create class with vectorizer=none (we provide vectors manually)
	schema := map[string]any{
		"class":           s.className,
		"vectorizer":      "none",
		"vectorIndexType": "hnsw",
		"vectorIndexConfig": map[string]any{
			"ef":              256,
			"efConstruction":  128,
			"maxConnections":  64,
			"dynamicEfFactor": 8,
			"dynamicEfMin":    100,
			"distance":        "cosine",
		},
		"properties": []map[string]any{
			{"name": "memory_id", "dataType": []string{"text"}, "tokenization": "field"},
			{"name": "memory_type", "dataType": []string{"text"}},
			{"name": "memory_target", "dataType": []string{"text"}},
			{"name": "content", "dataType": []string{"text"}},
			{"name": "when_to_use", "dataType": []string{"text"}},
			{"name": "author", "dataType": []string{"text"}},
			{"name": "ref_memory_id", "dataType": []string{"text"}},
			{"name": "score", "dataType": []string{"number"}},
			{"name": "metadata", "dataType": []string{"text"}},
		},
	}
	resp, err = s.doJSON(ctx, http.MethodPost, s.apiURL("/schema"), schema)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("weaviate: failed to create schema, status %d", resp.StatusCode)
	}
	return nil
}

// Insert implements VectorStore.
func (s *WeaviateVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	for _, n := range nodes {
		vec, err := s.embed.Embed(ctx, n.EmbeddingContent())
		if err != nil {
			return err
		}
		metaJSON, _ := json.Marshal(n.Metadata)
		body := map[string]any{
			"class":  s.className,
			"vector": vec,
			"properties": map[string]any{
				"memory_id":     n.MemoryID,
				"memory_type":   string(n.MemoryType),
				"memory_target": n.MemoryTarget,
				"content":       n.Content,
				"when_to_use":   n.WhenToUse,
				"author":        n.Author,
				"ref_memory_id": n.RefMemoryID,
				"score":         n.Score,
				"metadata":      string(metaJSON),
			},
		}
		resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/objects"), body)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("weaviate: insert failed, status %d", resp.StatusCode)
		}
	}
	return nil
}

// Search implements VectorStore.
func (s *WeaviateVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	vec, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	body := map[string]any{
		"vector": vec,
		"limit":  topK,
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL(fmt.Sprintf("/objects/%s/searches/nearVector", s.className)), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("weaviate: search failed, status %d", resp.StatusCode)
	}

	var result struct {
		Objects []struct {
			Properties map[string]any `json:"properties"`
			Vector     []float32      `json:"vector"`
			Certainty  float64        `json:"certainty"`
		} `json:"objects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	nodes := make([]*MemoryNode, 0, len(result.Objects))
	for _, obj := range result.Objects {
		props := obj.Properties
		node := &MemoryNode{
			MemoryID:     getStringProp(props, "memory_id"),
			MemoryType:   MemoryType(getStringProp(props, "memory_type")),
			MemoryTarget: getStringProp(props, "memory_target"),
			Content:      getStringProp(props, "content"),
			WhenToUse:    getStringProp(props, "when_to_use"),
			Author:       getStringProp(props, "author"),
			RefMemoryID:  getStringProp(props, "ref_memory_id"),
			Score:        obj.Certainty,
			Vector:       obj.Vector,
		}
		if metaStr := getStringProp(props, "metadata"); metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &node.Metadata)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// Get implements VectorStore.
func (s *WeaviateVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	// Use GraphQL or filter search
	body := map[string]any{
		"query": map[string]any{
			"operator":  "Equal",
			"path":      []string{"memory_id"},
			"valueText": memoryID,
		},
		"limit": 1,
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL(fmt.Sprintf("/objects/%s/searches", s.className)), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Objects []struct {
			Properties map[string]any `json:"properties"`
			Vector     []float32      `json:"vector"`
		} `json:"objects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Objects) == 0 {
		return nil, ErrMemoryNotFound
	}
	obj := result.Objects[0]
	props := obj.Properties
	node := &MemoryNode{
		MemoryID:     getStringProp(props, "memory_id"),
		MemoryType:   MemoryType(getStringProp(props, "memory_type")),
		MemoryTarget: getStringProp(props, "memory_target"),
		Content:      getStringProp(props, "content"),
		WhenToUse:    getStringProp(props, "when_to_use"),
		Author:       getStringProp(props, "author"),
		RefMemoryID:  getStringProp(props, "ref_memory_id"),
		Vector:       obj.Vector,
	}
	if metaStr := getStringProp(props, "metadata"); metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &node.Metadata)
	}
	return node, nil
}

// Update implements VectorStore.
func (s *WeaviateVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	// Weaviate doesn't have a direct update; delete + re-insert
	_ = s.Delete(ctx, node.MemoryID)
	return s.Insert(ctx, []*MemoryNode{node})
}

// Delete implements VectorStore.
func (s *WeaviateVectorStore) Delete(ctx context.Context, memoryID string) error {
	// Find object by memory_id filter, then delete by UUID
	body := map[string]any{
		"query": map[string]any{
			"operator":  "Equal",
			"path":      []string{"memory_id"},
			"valueText": memoryID,
		},
		"limit": 1,
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL(fmt.Sprintf("/objects/%s/searches", s.className)), body)
	if err != nil {
		return err
	}
	var result struct {
		Objects []struct {
			ID string `json:"id"`
		} `json:"objects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()
	if len(result.Objects) == 0 {
		return ErrMemoryNotFound
	}
	uuid := result.Objects[0].ID

	resp, err = s.doJSON(ctx, http.MethodDelete, s.apiURL("/objects/"+uuid), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// DeleteAll implements VectorStore.
func (s *WeaviateVectorStore) DeleteAll(ctx context.Context) error {
	// Delete all objects of this class
	resp, err := s.doJSON(ctx, http.MethodDelete, s.apiURL(fmt.Sprintf("/schema/%s", s.className)), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	// Recreate schema
	return s.ensureSchema(ctx)
}

func getStringProp(props map[string]any, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

var _ VectorStore = (*WeaviateVectorStore)(nil)
