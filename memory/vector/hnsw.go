package vector

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// HNSWIndex 实现 Hierarchical Navigable Small World 近似最近邻搜索索引。
// 基于 Malkov & Yashunin 2018 论文，支持增量插入和并发搜索。
type HNSWIndex struct {
	mu sync.RWMutex

	// 配置参数
	M        int     // 每层最大邻居数
	Mmax     int     // 第0层最大邻居数（通常 M*2）
	Mmax0    int     // 第0层最大邻居数
	efConstruction int // 构建时的搜索深度
	efSearch int     // 搜索时的搜索深度
	ml       float64 // 层级因子（越小层级越多）

	// 数据
	dim      int
	embed    EmbeddingModel
	nodes    map[string]*hnswNode // memoryID -> node
	entryPoint string             // 入口点
	maxLevel int                  // 当前最大层级
	rng      *rand.Rand
}

// hnswNode HNSW 节点
type hnswNode struct {
	id     string
	vector []float32
	level  int
	// neighbors[level] = []neighborID
	neighbors [][]string
}

// NewHNSWIndex 创建 HNSW 索引
func NewHNSWIndex(dim int, embed EmbeddingModel) *HNSWIndex {
	return &HNSWIndex{
		M:              16,
		Mmax:           32,
		Mmax0:          32,
		efConstruction: 200,
		efSearch:       64,
		ml:             1.0 / math.Log(16.0),
		dim:            dim,
		embed:          embed,
		nodes:          make(map[string]*hnswNode),
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Insert 插入节点到 HNSW 索引
func (h *HNSWIndex) Insert(ctx context.Context, id string, vector []float32) error {
	if h == nil {
		return errors.New("hnsw: nil index")
	}
	if len(vector) != h.dim {
		return ErrVectorDimension
	}

	// 计算层级
	level := h.randomLevel()

	node := &hnswNode{
		id:        id,
		vector:    make([]float32, len(vector)),
		level:     level,
		neighbors: make([][]string, level+1),
	}
	copy(node.vector, vector)

	h.mu.Lock()
	defer h.mu.Unlock()

	// 如果是第一个节点，设为入口点
	if len(h.nodes) == 0 {
		h.nodes[id] = node
		h.entryPoint = id
		h.maxLevel = level
		return nil
	}

	// 从入口点开始搜索
	entry := h.entryPoint
	entryNode := h.nodes[entry]
	if entryNode == nil {
		return errors.New("hnsw: entry point not found")
	}

	// 找到每层的最近邻居
	for lc := h.maxLevel; lc > level; lc-- {
		entry = h.searchLayerNearest(entry, vector, lc)
	}

	// 在 [0, level] 层插入邻居
	for lc := 0; lc <= level && lc <= h.maxLevel; lc++ {
		// 找到 efConstruction 个最近邻居
		neighbors := h.searchLayerKNN(entry, vector, lc, h.efConstruction)
		// 选择 M 个邻居（双向连接）
		selected := h.selectNeighbors(neighbors, h.M)
		node.neighbors[lc] = selected
		// 双向连接：更新邻居的邻居列表
		for _, nid := range selected {
			n := h.nodes[nid]
			if n == nil {
				continue
			}
			if lc == 0 {
				n.neighbors[lc] = h.addNeighbor(n.neighbors[lc], id, h.Mmax0)
			} else {
				n.neighbors[lc] = h.addNeighbor(n.neighbors[lc], id, h.Mmax)
			}
		}
		// 更新入口点
		if len(neighbors) > 0 {
			entry = neighbors[0].id
		}
	}

	h.nodes[id] = node
	if level > h.maxLevel {
		h.maxLevel = level
		h.entryPoint = id
	}
	return nil
}

// Search 搜索 K 个最近邻
func (h *HNSWIndex) Search(ctx context.Context, vector []float32, k int) ([]*hnswResult, error) {
	if h == nil || len(h.nodes) == 0 {
		return nil, nil
	}
	if len(vector) != h.dim {
		return nil, ErrVectorDimension
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	entry := h.entryPoint
	entryNode := h.nodes[entry]
	if entryNode == nil {
		return nil, errors.New("hnsw: entry point not found")
	}

	// 从最高层开始
	for lc := h.maxLevel; lc > 0; lc-- {
		entry = h.searchLayerNearest(entry, vector, lc)
	}

	// 在第0层搜索 K 个最近邻
	results := h.searchLayerKNN(entry, vector, 0, max(k, h.efSearch))
	if len(results) > k {
		results = results[:k]
	}
	return results, nil
}

// Delete 从索引中删除节点
func (h *HNSWIndex) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, ok := h.nodes[id]
	if !ok {
		return nil
	}

	// 从所有邻居的邻居列表中移除
	for lc, neighbors := range node.neighbors {
		for _, nid := range neighbors {
			n := h.nodes[nid]
			if n == nil {
				continue
			}
			n.neighbors[lc] = h.removeNeighbor(n.neighbors[lc], id)
		}
	}

	delete(h.nodes, id)

	// 如果删除的是入口点，需要重新选择
	if h.entryPoint == id && len(h.nodes) > 0 {
		for nid := range h.nodes {
			h.entryPoint = nid
			break
		}
	}
	return nil
}

// searchLayerNearest 在指定层搜索最近的一个邻居
func (h *HNSWIndex) searchLayerNearest(entryID string, vector []float32, level int) string {
	curr := entryID
	currDist := h.distance(vector, h.nodes[curr].vector)
	changed := true

	for changed {
		changed = false
		node := h.nodes[curr]
		if node == nil || level >= len(node.neighbors) {
			break
		}
		for _, nid := range node.neighbors[level] {
			n := h.nodes[nid]
			if n == nil {
				continue
			}
			d := h.distance(vector, n.vector)
			if d < currDist {
				currDist = d
				curr = nid
				changed = true
			}
		}
	}
	return curr
}

// searchLayerKNN 在指定层搜索 K 个最近邻居
func (h *HNSWIndex) searchLayerKNN(entryID string, vector []float32, level int, k int) []*hnswResult {
	// 使用优先队列（最小堆）维护候选集
	visited := make(map[string]bool)
	candidates := &hnswMinHeap{}
	results := &hnswMaxHeap{}

	entry := h.nodes[entryID]
	if entry == nil {
		return nil
	}

	d := h.distance(vector, entry.vector)
	candidates.push(&hnswResult{id: entryID, dist: d})
	results.push(&hnswResult{id: entryID, dist: d})
	visited[entryID] = true

	for candidates.len() > 0 {
		curr := candidates.pop()
		if curr.dist > results.peek().dist && results.len() >= k {
			break
		}

		node := h.nodes[curr.id]
		if node == nil || level >= len(node.neighbors) {
			continue
		}
		for _, nid := range node.neighbors[level] {
			if visited[nid] {
				continue
			}
			visited[nid] = true
			n := h.nodes[nid]
			if n == nil {
				continue
			}
			d := h.distance(vector, n.vector)
			if d < results.peek().dist || results.len() < k {
				candidates.push(&hnswResult{id: nid, dist: d})
				results.push(&hnswResult{id: nid, dist: d})
				if results.len() > k {
					results.pop()
				}
			}
		}
	}

	// 转换为有序结果（从小到大）
	var out []*hnswResult
	for results.len() > 0 {
		out = append(out, results.pop())
	}
	// 反转（从最近到最远）
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// selectNeighbors 从候选集中选择 M 个邻居（按距离排序）
func (h *HNSWIndex) selectNeighbors(candidates []*hnswResult, m int) []string {
	if len(candidates) > m {
		candidates = candidates[:m]
	}
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.id
	}
	return ids
}

// addNeighbor 添加邻居，保持列表长度不超过 maxM
func (h *HNSWIndex) addNeighbor(neighbors []string, id string, maxM int) []string {
	for _, nid := range neighbors {
		if nid == id {
			return neighbors
		}
	}
	neighbors = append(neighbors, id)
	if len(neighbors) > maxM {
		// 简单截断（实际应基于距离选择）
		neighbors = neighbors[:maxM]
	}
	return neighbors
}

// removeNeighbor 从邻居列表中移除指定 ID
func (h *HNSWIndex) removeNeighbor(neighbors []string, id string) []string {
	var out []string
	for _, nid := range neighbors {
		if nid != id {
			out = append(out, nid)
		}
	}
	return out
}

// distance 计算欧氏距离（用于 HNSW 搜索）
func (h *HNSWIndex) distance(a, b []float32) float64 {
	var sum float64
	for i := range a {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

// randomLevel 生成随机层级
func (h *HNSWIndex) randomLevel() int {
	level := 0
	for h.rng.Float64() < h.ml && level < 16 {
		level++
	}
	return level
}

// hnswResult 搜索结果
type hnswResult struct {
	id   string
	dist float64
}

// hnswMinHeap 最小堆（用于候选集）
type hnswMinHeap struct {
	items []*hnswResult
}

func (h *hnswMinHeap) push(item *hnswResult) {
	h.items = append(h.items, item)
	// 上浮
	i := len(h.items) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if h.items[parent].dist <= h.items[i].dist {
			break
		}
		h.items[parent], h.items[i] = h.items[i], h.items[parent]
		i = parent
	}
}

func (h *hnswMinHeap) pop() *hnswResult {
	if len(h.items) == 0 {
		return nil
	}
	root := h.items[0]
	last := len(h.items) - 1
	h.items[0] = h.items[last]
	h.items = h.items[:last]

	// 下沉
	i := 0
	for {
		left := 2*i + 1
		right := 2*i + 2
		smallest := i
		if left < len(h.items) && h.items[left].dist < h.items[smallest].dist {
			smallest = left
		}
		if right < len(h.items) && h.items[right].dist < h.items[smallest].dist {
			smallest = right
		}
		if smallest == i {
			break
		}
		h.items[i], h.items[smallest] = h.items[smallest], h.items[i]
		i = smallest
	}
	return root
}

func (h *hnswMinHeap) len() int {
	return len(h.items)
}

// hnswMaxHeap 最大堆（用于结果集）
type hnswMaxHeap struct {
	items []*hnswResult
}

func (h *hnswMaxHeap) push(item *hnswResult) {
	h.items = append(h.items, item)
	// 上浮（按距离最大）
	i := len(h.items) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if h.items[parent].dist >= h.items[i].dist {
			break
		}
		h.items[parent], h.items[i] = h.items[i], h.items[parent]
		i = parent
	}
}

func (h *hnswMaxHeap) pop() *hnswResult {
	if len(h.items) == 0 {
		return nil
	}
	root := h.items[0]
	last := len(h.items) - 1
	h.items[0] = h.items[last]
	h.items = h.items[:last]

	// 下沉
	i := 0
	for {
		left := 2*i + 1
		right := 2*i + 2
		largest := i
		if left < len(h.items) && h.items[left].dist > h.items[largest].dist {
			largest = left
		}
		if right < len(h.items) && h.items[right].dist > h.items[largest].dist {
			largest = right
		}
		if largest == i {
			break
		}
		h.items[i], h.items[largest] = h.items[largest], h.items[i]
		i = largest
	}
	return root
}

func (h *hnswMaxHeap) peek() *hnswResult {
	if len(h.items) == 0 {
		return &hnswResult{dist: math.MaxFloat64}
	}
	return h.items[0]
}

func (h *hnswMaxHeap) len() int {
	return len(h.items)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
