package vector

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
)

// LocalVectorStore 内存向量库（余弦相似度 + 简单过滤）
type LocalVectorStore struct {
	mu    sync.RWMutex
	dim   int
	embed EmbeddingModel
	nodes map[string]*MemoryNode
}

// NewLocalVectorStore 创建本地存储；dimension 在首次 Insert 时由向量长度确定
func NewLocalVectorStore(embed EmbeddingModel) *LocalVectorStore {
	return &LocalVectorStore{
		embed: embed,
		nodes: make(map[string]*MemoryNode),
	}
}

// Insert 写入节点；若 Vector 为空则调用嵌入模型
func (s *LocalVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s == nil || s.embed == nil {
		return ErrEmbeddingRequired
	}
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
		if s.dim == 0 {
			s.dim = len(node.Vector)
		}
		if len(node.Vector) != s.dim {
			return ErrVectorDimension
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, node := range nodes {
		if node == nil {
			continue
		}
		s.nodes[node.MemoryID] = node
	}
	return nil
}

// Search 按查询文本嵌入后做相似度检索
func (s *LocalVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if s == nil || s.embed == nil {
		return nil, ErrEmbeddingRequired
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	type scored struct {
		n *MemoryNode
		f float64
	}
	var out []scored
	for _, n := range s.nodes {
		if !matchesRetrieveFilter(n, opts) {
			continue
		}
		if len(n.Vector) != len(qv) {
			continue
		}
		sim := CosineSimilarity(qv, n.Vector)
		if sim < opts.MinScore {
			continue
		}
		nn := *n
		nn.Score = sim
		out = append(out, scored{&nn, sim})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].f > out[j].f })
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}
	if topK > len(out) {
		topK = len(out)
	}
	res := make([]*MemoryNode, topK)
	for i := 0; i < topK; i++ {
		res[i] = out[i].n
	}
	return res, nil
}

func matchesRetrieveFilter(n *MemoryNode, opts RetrieveOptions) bool {
	if len(opts.MemoryTypes) > 0 {
		ok := false
		for _, t := range opts.MemoryTypes {
			if n.MemoryType == t {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(opts.MemoryTargets) > 0 {
		ok := false
		for _, t := range opts.MemoryTargets {
			if n.MemoryTarget == t {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// Get 按键读取
func (s *LocalVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[memoryID]
	if !ok {
		return nil, ErrMemoryNotFound
	}
	return n, nil
}

// Update 覆盖
func (s *LocalVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	_ = ctx
	if node == nil || node.MemoryID == "" {
		return ErrInvalidMemoryNode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[node.MemoryID] = node
	return nil
}

// Delete 删除
func (s *LocalVectorStore) Delete(ctx context.Context, memoryID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, memoryID)
	return nil
}

// DeleteAll 清空
func (s *LocalVectorStore) DeleteAll(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = make(map[string]*MemoryNode)
	return nil
}

// List 按类型和目标过滤列出节点，limit<=0 表示不限制
func (s *LocalVectorStore) List(memType MemoryType, target string, limit int) ([]*MemoryNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*MemoryNode
	for _, n := range s.nodes {
		if n.MemoryType != memType {
			continue
		}
		if target != "" && n.MemoryTarget != target {
			continue
		}
		out = append(out, n)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// BatchSearchWithThreshold 跨多查询计算平均余弦相似度，按阈值过滤并排序。
// queries 是已去重的查询字符串列表，candidates 是按 memoryID 索引的候选节点。
func (s *LocalVectorStore) BatchSearchWithThreshold(queryStrings []string, candidates map[string]*MemoryNode, hybridThreshold float64) []*MemoryNode {
	if s.embed == nil || len(queryStrings) == 0 || len(candidates) == 0 {
		result := make([]*MemoryNode, 0, len(candidates))
		for _, n := range candidates {
			result = append(result, n)
		}
		return result
	}

	// 收集所有候选节点（有向量）
	var nodesWithVector []*MemoryNode
	for _, n := range candidates {
		if len(n.Vector) > 0 {
			nodesWithVector = append(nodesWithVector, n)
		}
	}
	if len(nodesWithVector) == 0 {
		result := make([]*MemoryNode, 0, len(candidates))
		for _, n := range candidates {
			result = append(result, n)
		}
		return result
	}

	// 对每个 query 计算嵌入，然后计算与所有候选节点的余弦相似度
	type scoredNode struct {
		n     *MemoryNode
		score float64
	}

	cumScores := make(map[string]float64, len(nodesWithVector))
	for _, n := range nodesWithVector {
		cumScores[n.MemoryID] = 0
	}

	queryCount := 0
	for _, qs := range queryStrings {
		qv, err := s.embed.Embed(context.Background(), qs)
		if err != nil {
			continue
		}
		for _, n := range nodesWithVector {
			if len(qv) != len(n.Vector) {
				continue
			}
			sim := CosineSimilarity(qv, n.Vector)
			cumScores[n.MemoryID] += sim
		}
		queryCount++
	}

	if queryCount == 0 {
		result := make([]*MemoryNode, 0, len(candidates))
		for _, n := range candidates {
			result = append(result, n)
		}
		return result
	}

	var scored []scoredNode
	for _, n := range nodesWithVector {
		avg := cumScores[n.MemoryID] / float64(queryCount)
		if avg >= hybridThreshold {
			n.Score = avg
			scored = append(scored, scoredNode{n: n, score: avg})
		}
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	result := make([]*MemoryNode, len(scored))
	for i, sc := range scored {
		result[i] = sc.n
	}
	return result
}

// ErrEmbeddingRequired 未配置嵌入模型
var ErrEmbeddingRequired = errors.New("memory: embedding model required")

// ErrVectorDimension 向量维度不一致
var ErrVectorDimension = errors.New("memory: vector dimension mismatch")

// ErrMemoryNotFound 未找到
var ErrMemoryNotFound = errors.New("memory: not found")

// ErrInvalidMemoryNode 无效节点
var ErrInvalidMemoryNode = errors.New("memory: invalid memory node")

func (s *LocalVectorStore) WriteSnapshot(path string) error {
	if s == nil {
		return errors.New("memory: nil LocalVectorStore")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.MarshalIndent(s.nodes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *LocalVectorStore) ReadSnapshot(path string) error {
	if s == nil {
		return errors.New("memory: nil LocalVectorStore")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var nodes map[string]*MemoryNode
	if err := json.Unmarshal(data, &nodes); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = nodes
	if len(nodes) > 0 {
		for _, n := range nodes {
			if len(n.Vector) > 0 {
				s.dim = len(n.Vector)
				break
			}
		}
	}
	return nil
}
