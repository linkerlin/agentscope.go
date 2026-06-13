package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MilvusVectorStore 基于 Milvus REST API (v2) 的远程向量存储实现。
type MilvusVectorStore struct {
	baseURL    string
	collection string
	dim        int
	embed      EmbeddingModel
	client     *http.Client
}

// NewMilvusVectorStore 创建 Milvus 向量存储。
// baseURL 默认 http://localhost:19530
func NewMilvusVectorStore(baseURL, collection string, dim int, embed EmbeddingModel) (*MilvusVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if baseURL == "" {
		baseURL = "http://localhost:19530"
	}
	if collection == "" {
		collection = "agentscope_memory"
	}
	s := &MilvusVectorStore{
		baseURL:    baseURL,
		collection: collection,
		dim:        dim,
		embed:      embed,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
	return s, nil
}

func (s *MilvusVectorStore) doJSON(ctx context.Context, method, url string, body any) (*http.Response, error) {
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

func (s *MilvusVectorStore) apiURL(path string) string {
	return fmt.Sprintf("%s/v2/vectordb%s", s.baseURL, path)
}

func (s *MilvusVectorStore) ensureCollection(ctx context.Context) error {
	body := map[string]any{
		"collectionName": s.collection,
		"dimension":      s.dim,
		"metricType":     "COSINE",
		"idType":         "VarChar",
		"params": map[string]any{
			"max_length": 64,
		},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/collections/create"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("milvus: create collection failed, status %d", resp.StatusCode)
	}
	return nil
}

// Insert 插入记忆节点。
func (s *MilvusVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s.embed == nil {
		return ErrEmbeddingRequired
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	var ids []string
	var vectors [][]float32
	var contents []string
	var memoryTypes []string
	var memoryTargets []string
	var whenToUses []string
	var authors []string
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
		ids = append(ids, node.MemoryID)
		vectors = append(vectors, node.Vector)
		contents = append(contents, node.Content)
		memoryTypes = append(memoryTypes, string(node.MemoryType))
		memoryTargets = append(memoryTargets, node.MemoryTarget)
		whenToUses = append(whenToUses, node.WhenToUse)
		authors = append(authors, node.Author)
	}
	if len(ids) == 0 {
		return nil
	}
	body := map[string]any{
		"collectionName": s.collection,
		"data": []map[string]any{
			{
				"id":            ids,
				"vector":        vectors,
				"content":       contents,
				"memory_type":   memoryTypes,
				"memory_target": memoryTargets,
				"when_to_use":   whenToUses,
				"author":        authors,
			},
		},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/entities/insert"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("milvus: insert failed, status %d", resp.StatusCode)
	}
	return nil
}

// Search 语义检索。
func (s *MilvusVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
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
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}
	body := map[string]any{
		"collectionName": s.collection,
		"data":           [][]float32{qv},
		"annsField":      "vector",
		"limit":          topK,
		"outputFields":   []string{"id", "content", "memory_type", "memory_target", "when_to_use", "author"},
	}
	if opts.MinScore > 0 {
		body["params"] = map[string]any{"radius": opts.MinScore}
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/entities/search"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("milvus: search failed, status %d", resp.StatusCode)
	}
	var result milvusSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.toNodes(opts.MinScore), nil
}

// Get 按 memoryID 读取。
func (s *MilvusVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	body := map[string]any{
		"collectionName": s.collection,
		"id":             []string{memoryID},
		"outputFields":   []string{"id", "content", "memory_type", "memory_target", "when_to_use", "author"},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/entities/get"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("milvus: get failed, status %d", resp.StatusCode)
	}
	var result milvusGetResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	nodes := result.toNodes()
	if len(nodes) == 0 {
		return nil, ErrMemoryNotFound
	}
	return nodes[0], nil
}

// Update 覆盖更新。
func (s *MilvusVectorStore) Update(ctx context.Context, node *MemoryNode) error {
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
		"collectionName": s.collection,
		"data": []map[string]any{
			{
				"id":            node.MemoryID,
				"vector":        node.Vector,
				"content":       node.Content,
				"memory_type":   string(node.MemoryType),
				"memory_target": node.MemoryTarget,
				"when_to_use":   node.WhenToUse,
				"author":        node.Author,
			},
		},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/entities/upsert"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("milvus: upsert failed, status %d", resp.StatusCode)
	}
	return nil
}

// Delete 按 memoryID 删除。
func (s *MilvusVectorStore) Delete(ctx context.Context, memoryID string) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"collectionName": s.collection,
		"id":             []string{memoryID},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/entities/delete"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("milvus: delete failed, status %d", resp.StatusCode)
	}
	return nil
}

// DeleteAll 删除集合并重建。
func (s *MilvusVectorStore) DeleteAll(ctx context.Context) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	body := map[string]any{"collectionName": s.collection}
	resp, err := s.doJSON(ctx, http.MethodPost, s.apiURL("/collections/drop"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return s.ensureCollection(ctx)
}

type milvusSearchResult struct {
	Data [][]milvusResultItem `json:"data"`
}

type milvusResultItem struct {
	ID           string  `json:"id"`
	Content      string  `json:"content"`
	MemoryType   string  `json:"memory_type"`
	MemoryTarget string  `json:"memory_target"`
	WhenToUse    string  `json:"when_to_use"`
	Author       string  `json:"author"`
	Distance     float64 `json:"distance"`
}

func (r *milvusSearchResult) toNodes(minScore float64) []*MemoryNode {
	if len(r.Data) == 0 {
		return nil
	}
	var nodes []*MemoryNode
	for _, item := range r.Data[0] {
		sim := 1.0 - item.Distance
		if sim < minScore {
			continue
		}
		nodes = append(nodes, &MemoryNode{
			MemoryID:     item.ID,
			Content:      item.Content,
			MemoryType:   MemoryType(item.MemoryType),
			MemoryTarget: item.MemoryTarget,
			WhenToUse:    item.WhenToUse,
			Author:       item.Author,
			Score:        sim,
		})
	}
	return nodes
}

type milvusGetResult struct {
	Data []milvusResultItem `json:"data"`
}

func (r *milvusGetResult) toNodes() []*MemoryNode {
	var nodes []*MemoryNode
	for _, item := range r.Data {
		nodes = append(nodes, &MemoryNode{
			MemoryID:     item.ID,
			Content:      item.Content,
			MemoryType:   MemoryType(item.MemoryType),
			MemoryTarget: item.MemoryTarget,
			WhenToUse:    item.WhenToUse,
			Author:       item.Author,
		})
	}
	return nodes
}

var _ VectorStore = (*MilvusVectorStore)(nil)
