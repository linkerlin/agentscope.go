package memory

import (
	"context"
	"errors"
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
			v, err := s.embed.Embed(ctx, node.Content)
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

// ErrEmbeddingRequired 未配置嵌入模型
var ErrEmbeddingRequired = errors.New("memory: embedding model required")

// ErrVectorDimension 向量维度不一致
var ErrVectorDimension = errors.New("memory: vector dimension mismatch")

// ErrMemoryNotFound 未找到
var ErrMemoryNotFound = errors.New("memory: not found")

// ErrInvalidMemoryNode 无效节点
var ErrInvalidMemoryNode = errors.New("memory: invalid memory node")
