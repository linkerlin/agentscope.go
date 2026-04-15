package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ChromaVectorStore 基于 Chroma REST API (v1) 的远程向量存储实现
type ChromaVectorStore struct {
	baseURL    string
	collection string
	embed      EmbeddingModel
	dim        int
	client     *http.Client
}

// NewChromaVectorStore 创建 Chroma 向量存储
func NewChromaVectorStore(baseURL, collection string, dim int, embed EmbeddingModel) (*ChromaVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	s := &ChromaVectorStore{
		baseURL:    baseURL,
		collection: collection,
		embed:      embed,
		dim:        dim,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
	if err := s.ensureCollection(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ChromaVectorStore) ensureCollection(ctx context.Context) error {
	// 尝试获取集合，若 404 则创建
	reqURL := fmt.Sprintf("%s/api/v1/collections/%s", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
	// 创建集合
	body := map[string]any{
		"name": s.collection,
		"metadata": map[string]any{
			"hnsw:space": "cosine",
		},
	}
	data, _ := json.Marshal(body)
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/collections", s.baseURL), bytes.NewReader(data))
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
		return fmt.Errorf("chroma: failed to create collection, status %d", resp.StatusCode)
	}
	return nil
}

func (s *ChromaVectorStore) collectionURL(action string) string {
	return fmt.Sprintf("%s/api/v1/collections/%s/%s", s.baseURL, s.collection, action)
}

func (s *ChromaVectorStore) doJSON(ctx context.Context, method, url string, body any) (*http.Response, error) {
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
func (s *ChromaVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s.embed == nil {
		return ErrEmbeddingRequired
	}
	var ids []string
	var embeddings [][]float32
	var metadatas []map[string]any
	var documents []string
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
		ids = append(ids, node.MemoryID)
		embeddings = append(embeddings, node.Vector)
		metadatas = append(metadatas, nodeToChromaMetadata(node))
		documents = append(documents, node.Content)
	}
	if len(ids) == 0 {
		return nil
	}
	body := map[string]any{
		"ids":        ids,
		"embeddings": embeddings,
		"metadatas":  metadatas,
		"documents":  documents,
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.collectionURL("add"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("chroma: add failed, status %d", resp.StatusCode)
	}
	return nil
}

// Search 语义检索
func (s *ChromaVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if s.embed == nil {
		return nil, ErrEmbeddingRequired
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	nResults := opts.TopK
	if nResults <= 0 {
		nResults = 10
	}
	body := map[string]any{
		"query_embeddings": [][]float32{qv},
		"n_results":        nResults,
		"include":          []string{"metadatas", "documents", "distances", "embeddings"},
	}
	where := buildChromaWhere(opts)
	if len(where) > 0 {
		body["where"] = where
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.collectionURL("query"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chroma: query failed, status %d", resp.StatusCode)
	}
	var result chromaQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.toNodes(opts.MinScore), nil
}

// Get 按 memoryID 读取
func (s *ChromaVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	body := map[string]any{
		"ids":     []string{memoryID},
		"include": []string{"metadatas", "documents", "embeddings"},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.collectionURL("get"), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chroma: get failed, status %d", resp.StatusCode)
	}
	var result chromaGetResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	nodes := result.toNodes()
	if len(nodes) == 0 {
		return nil, ErrMemoryNotFound
	}
	return nodes[0], nil
}

// Update 覆盖更新
func (s *ChromaVectorStore) Update(ctx context.Context, node *MemoryNode) error {
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
	body := map[string]any{
		"ids":        []string{node.MemoryID},
		"embeddings": [][]float32{node.Vector},
		"metadatas":  []map[string]any{nodeToChromaMetadata(node)},
		"documents":  []string{node.Content},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.collectionURL("update"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("chroma: update failed, status %d", resp.StatusCode)
	}
	return nil
}

// Delete 按 memoryID 删除
func (s *ChromaVectorStore) Delete(ctx context.Context, memoryID string) error {
	body := map[string]any{
		"ids": []string{memoryID},
	}
	resp, err := s.doJSON(ctx, http.MethodPost, s.collectionURL("delete"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("chroma: delete failed, status %d", resp.StatusCode)
	}
	return nil
}

// DeleteAll 清空集合
func (s *ChromaVectorStore) DeleteAll(ctx context.Context) error {
	body := map[string]any{"where": map[string]any{}}
	resp, err := s.doJSON(ctx, http.MethodPost, s.collectionURL("delete"), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("chroma: delete_all failed, status %d", resp.StatusCode)
	}
	return nil
}

func nodeToChromaMetadata(node *MemoryNode) map[string]any {
	m := map[string]any{
		"memory_type":   string(node.MemoryType),
		"memory_target": node.MemoryTarget,
		"when_to_use":   node.WhenToUse,
		"author":        node.Author,
		"ref_memory_id": node.RefMemoryID,
	}
	if !node.TimeCreated.IsZero() {
		m["time_created"] = node.TimeCreated.Format(time.RFC3339)
	}
	if !node.TimeModified.IsZero() {
		m["time_modified"] = node.TimeModified.Format(time.RFC3339)
	}
	if !node.MessageTime.IsZero() {
		m["message_time"] = node.MessageTime.Format(time.RFC3339)
	}
	return m
}

func buildChromaWhere(opts RetrieveOptions) map[string]any {
	where := make(map[string]any)
	if len(opts.MemoryTypes) > 0 {
		types := make([]string, len(opts.MemoryTypes))
		for i, t := range opts.MemoryTypes {
			types[i] = string(t)
		}
		where["memory_type"] = map[string]any{"$in": types}
	}
	if len(opts.MemoryTargets) > 0 {
		if len(opts.MemoryTargets) == 1 {
			where["memory_target"] = map[string]any{"$eq": opts.MemoryTargets[0]}
		} else {
			where["memory_target"] = map[string]any{"$in": opts.MemoryTargets}
		}
	}
	return where
}

// chromaQueryResult 解析 Chroma /query 返回
type chromaQueryResult struct {
	IDs        [][]string           `json:"ids"`
	Documents  [][]string           `json:"documents"`
	Metadatas  [][]map[string]any   `json:"metadatas"`
	Distances  [][]float64          `json:"distances"`
	Embeddings [][][]float32        `json:"embeddings"`
}

func (r *chromaQueryResult) toNodes(minScore float64) []*MemoryNode {
	if len(r.IDs) == 0 {
		return nil
	}
	var nodes []*MemoryNode
	batch := r.IDs[0]
	for i, id := range batch {
		dist := 0.0
		if len(r.Distances) > 0 && len(r.Distances[0]) > i {
			dist = r.Distances[0][i]
		}
		// Chroma cosine distance = 1 - cosine_similarity, 转为相似度
		sim := 1.0 - dist
		if sim < minScore {
			continue
		}
		doc := ""
		if len(r.Documents) > 0 && len(r.Documents[0]) > i {
			doc = r.Documents[0][i]
		}
		var meta map[string]any
		if len(r.Metadatas) > 0 && len(r.Metadatas[0]) > i {
			meta = r.Metadatas[0][i]
		}
		var vec []float32
		if len(r.Embeddings) > 0 && len(r.Embeddings[0]) > i {
			vec = r.Embeddings[0][i]
		}
		n := chromaMetadataToNode(id, doc, meta, vec, sim)
		if n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// chromaGetResult 解析 Chroma /get 返回
type chromaGetResult struct {
	IDs       []string         `json:"ids"`
	Documents []string         `json:"documents"`
	Metadatas []map[string]any `json:"metadatas"`
	Embeddings [][]float32     `json:"embeddings"`
}

func (r *chromaGetResult) toNodes() []*MemoryNode {
	var nodes []*MemoryNode
	for i, id := range r.IDs {
		doc := ""
		if len(r.Documents) > i {
			doc = r.Documents[i]
		}
		var meta map[string]any
		if len(r.Metadatas) > i {
			meta = r.Metadatas[i]
		}
		var vec []float32
		if len(r.Embeddings) > i {
			vec = r.Embeddings[i]
		}
		n := chromaMetadataToNode(id, doc, meta, vec, 0)
		if n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func chromaMetadataToNode(id, document string, meta map[string]any, vec []float32, score float64) *MemoryNode {
	getStr := func(key string) string {
		if v, ok := meta[key]; ok {
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
	n := &MemoryNode{
		MemoryID:     id,
		Content:      document,
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
	return n
}

var _ VectorStore = (*ChromaVectorStore)(nil)
