package memory

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// MemoryCollector 基于使用频率和效用的记忆自动清理器。
// 对标 ReMe Python delete_task_memory 流水线。
// 增强版：支持 LRU/LFU/TTL 混合策略 + 自适应阈值 + 批量清理
type MemoryCollector struct {
	Store            VectorStore
	FreqThreshold    int           // 检索频率达到此阈值时才考虑删除
	UtilityThreshold float64       // utility/freq 低于此值则删除
	MaxAge           time.Duration // 最大保留时间，超时自动删除
	mu               sync.Mutex

	// 增强策略
	Strategy      GCMixedStrategy // 混合回收策略
	AdaptiveMode  bool            // 自适应阈值模式
	BatchSize     int             // 批量清理大小
	PreserveTypes []MemoryType    // 保护的记忆类型
	MinKeepCount  int             // 最少保留数量
}

// GCMixedStrategy 混合回收策略
type GCMixedStrategy struct {
	LRUWeight   float64 `json:"lru_weight"`   // 最近使用权重 (0-1)
	LFUWeight   float64 `json:"lfu_weight"`   // 频率使用权重 (0-1)
	TTLWeight   float64 `json:"ttl_weight"`   // 时间衰减权重 (0-1)
	ScoreWeight float64 `json:"score_weight"` // 质量分数权重 (0-1)
}

// DefaultGCMixedStrategy 返回默认混合策略
func DefaultGCMixedStrategy() GCMixedStrategy {
	return GCMixedStrategy{
		LRUWeight:   0.3,
		LFUWeight:   0.3,
		TTLWeight:   0.2,
		ScoreWeight: 0.2,
	}
}

// NewMemoryCollector 创建记忆清理器
func NewMemoryCollector(store VectorStore) *MemoryCollector {
	return &MemoryCollector{
		Store:            store,
		FreqThreshold:    3,
		UtilityThreshold: 0.2,
		MaxAge:           30 * 24 * time.Hour,
		Strategy:         DefaultGCMixedStrategy(),
		AdaptiveMode:     true,
		BatchSize:        100,
		PreserveTypes:    []MemoryType{MemoryTypePersonal, MemoryTypeHistory},
		MinKeepCount:     100,
	}
}

// RecordAccess 记录一次记忆被检索使用
func (c *MemoryCollector) RecordAccess(ctx context.Context, nodes []*MemoryNode) {
	for _, n := range nodes {
		if n == nil || n.MemoryID == "" {
			continue
		}
		// 更新频率
		freq := intVal(n.Metadata, "freq") + 1
		n.Metadata["freq"] = freq
		n.Metadata["last_accessed"] = time.Now().Unix()

		// 简单效用计算（基于 Score）
		if n.Score > 0 {
			prevUtil := floatVal(n.Metadata, "utility")
			prevFreq := float64(freq - 1)
			if prevFreq == 0 {
				prevFreq = 1
			}
			// 指数衰减平均: new_utility = alpha * score + (1-alpha) * old_utility
			alpha := 0.3
			newUtil := alpha*n.Score + (1-alpha)*prevUtil
			// 也基于累计效果: utility ~ sum(score * decay) / freq
			cumUtil := floatVal(n.Metadata, "cumulative_score") + n.Score
			n.Metadata["cumulative_score"] = cumUtil
			n.Metadata["utility"] = cumUtil / float64(freq)
			_ = prevUtil
			_ = newUtil
		}

		_ = c.Store.Update(ctx, n)
	}
}

// Collect 扫描并删除低价值记忆（增强版：混合策略）
// 返回被删除的 memoryID 列表
func (c *MemoryCollector) Collect(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Store == nil {
		return nil, nil
	}

	// 调用 list 获取全部节点
	nodes, err := c.Store.Search(ctx, "", RetrieveOptions{
		TopK:     10000,
		MinScore: 0,
	})
	if err != nil {
		return nil, err
	}

	// 如果总数低于最小保留数，不清理
	if len(nodes) <= c.MinKeepCount {
		return nil, nil
	}

	// 自适应阈值调整
	if c.AdaptiveMode {
		c.adjustThresholds(nodes)
	}

	// 计算每个节点的综合价值分数
	type scoredNode struct {
		node  *MemoryNode
		score float64 // 综合价值分数（越高越保留）
	}

	scored := make([]scoredNode, 0, len(nodes))
	now := time.Now()

	for _, n := range nodes {
		if n == nil || n.MemoryID == "" {
			continue
		}

		// 检查保护类型
		if c.isProtected(n) {
			continue
		}

		score := c.computeMixedScore(n, now)
		scored = append(scored, scoredNode{node: n, score: score})
	}

	// 按价值分数排序（低到高）
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
	})

	// 确定清理数量：保留 MinKeepCount + 高价值节点
	targetCount := c.MinKeepCount
	if len(scored) > targetCount {
		targetCount = len(scored) * 8 / 10 // 保留 80%
	}

	if targetCount < c.MinKeepCount {
		targetCount = c.MinKeepCount
	}

	var toDelete []string
	for i := 0; i < len(scored) && len(toDelete) < c.BatchSize; i++ {
		if i >= len(scored)-targetCount {
			break // 保留目标数量
		}

		n := scored[i].node

		// 额外检查：即使分数低，如果未过期且效用足够，保留
		freq := intVal(n.Metadata, "freq")
		utility := floatVal(n.Metadata, "utility")
		if c.MaxAge > 0 {
			lastAccess := int64Val(n.Metadata, "last_accessed")
			if lastAccess > 0 {
				age := now.Sub(time.Unix(lastAccess, 0))
				if age > c.MaxAge {
					toDelete = append(toDelete, n.MemoryID)
					continue
				}
			}
		}

		// 频率过高但效用过低：说明经常被检索但帮不上忙
		if freq >= c.FreqThreshold && utility < c.UtilityThreshold {
			toDelete = append(toDelete, n.MemoryID)
		}
	}

	for _, id := range toDelete {
		_ = c.Store.Delete(ctx, id)
	}

	return toDelete, nil
}

// CollectWithStrategy 使用指定策略执行垃圾回收
func (c *MemoryCollector) CollectWithStrategy(ctx context.Context, strategy GCStrategy) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Store == nil {
		return nil, nil
	}

	nodes, err := c.Store.Search(ctx, "", RetrieveOptions{
		TopK:     10000,
		MinScore: 0,
	})
	if err != nil {
		return nil, err
	}

	var toDelete []string
	now := time.Now()

	for _, n := range nodes {
		if n == nil || n.MemoryID == "" {
			continue
		}

		// 检查保护类型
		if c.isProtectedType(n, strategy.PreserveTypes) {
			continue
		}

		// 检查年龄
		if strategy.MaxAge > 0 {
			lastAccess := int64Val(n.Metadata, "last_accessed")
			if lastAccess > 0 {
				age := now.Sub(time.Unix(lastAccess, 0))
				if age > strategy.MaxAge {
					toDelete = append(toDelete, n.MemoryID)
					continue
				}
			}
		}

		// 检查最低分数
		if strategy.MinScore > 0 {
			score := n.Score
			if score < strategy.MinScore {
				toDelete = append(toDelete, n.MemoryID)
				continue
			}
		}

		// 检查最大数量
		if strategy.MaxCount > 0 && len(nodes)-len(toDelete) > strategy.MaxCount {
			// 按效用排序后删除最低的部分
			toDelete = append(toDelete, n.MemoryID)
		}
	}

	for _, id := range toDelete {
		_ = c.Store.Delete(ctx, id)
	}

	return toDelete, nil
}

// isProtected 检查节点是否受保护
func (c *MemoryCollector) isProtected(n *MemoryNode) bool {
	return c.isProtectedType(n, c.PreserveTypes)
}

// isProtectedType 检查节点类型是否在保护列表中
func (c *MemoryCollector) isProtectedType(n *MemoryNode, types []MemoryType) bool {
	if n == nil || n.MemoryType == "" {
		return false
	}
	for _, t := range types {
		if n.MemoryType == t {
			return true
		}
	}
	return false
}

// computeMixedScore 计算混合策略分数
func (c *MemoryCollector) computeMixedScore(n *MemoryNode, now time.Time) float64 {
	if n == nil {
		return 0
	}

	s := c.Strategy
	totalWeight := s.LRUWeight + s.LFUWeight + s.TTLWeight + s.ScoreWeight
	if totalWeight == 0 {
		totalWeight = 1
	}

	// LRU 分数：最近访问时间越近越高
	var lruScore float64
	if v, ok := n.Metadata["last_accessed"]; ok {
		if lastAccess, ok := v.(int64); ok && lastAccess > 0 {
			age := now.Sub(time.Unix(lastAccess, 0)).Hours()
			lruScore = 1.0 / (1.0 + age/24.0) // 24小时内为1，随时间衰减
		}
	}

	// LFU 分数：频率越高越高
	freq := float64(intVal(n.Metadata, "freq"))
	lfuScore := math.Min(freq/10.0, 1.0) // 10次为满分

	// TTL 分数：基于创建时间
	var ttlScore float64
	if v, ok := n.Metadata["created_at"]; ok {
		if created, ok := v.(int64); ok && created > 0 {
			age := now.Sub(time.Unix(created, 0)).Hours()
			ttlScore = 1.0 / (1.0 + age/168.0) // 7天为半衰期
		}
	}

	// Score 分数：原始质量分数
	scoreScore := n.Score

	// 加权综合
	mixed := (s.LRUWeight*lruScore + s.LFUWeight*lfuScore +
		s.TTLWeight*ttlScore + s.ScoreWeight*scoreScore) / totalWeight

	return mixed
}

// adjustThresholds 自适应调整阈值
func (c *MemoryCollector) adjustThresholds(nodes []*MemoryNode) {
	if len(nodes) == 0 {
		return
	}

	// 计算平均效用和频率
	var totalUtility, totalFreq float64
	var count int
	for _, n := range nodes {
		if n == nil {
			continue
		}
		utility := floatVal(n.Metadata, "utility")
		freq := float64(intVal(n.Metadata, "freq"))
		totalUtility += utility
		totalFreq += freq
		count++
	}

	if count == 0 {
		return
	}

	avgUtility := totalUtility / float64(count)
	avgFreq := totalFreq / float64(count)

	// 自适应调整：如果平均效用低，降低阈值；反之提高
	if avgUtility < 0.1 {
		c.UtilityThreshold = math.Max(0.05, c.UtilityThreshold*0.9)
	} else if avgUtility > 0.5 {
		c.UtilityThreshold = math.Min(0.8, c.UtilityThreshold*1.1)
	}

	// 频率阈值同理
	if avgFreq < 1 {
		c.FreqThreshold = int(math.Max(1, float64(c.FreqThreshold)*0.9))
	} else if avgFreq > 10 {
		c.FreqThreshold = int(math.Min(20, float64(c.FreqThreshold)*1.1))
	}
}

// EstimateUtility 估计记忆的效用值（0-1）
// 基于多个信号：分数、频率、时效性
func EstimateUtility(node *MemoryNode) float64 {
	if node == nil {
		return 0
	}

	freq := float64(intVal(node.Metadata, "freq"))
	utility := floatVal(node.Metadata, "utility")

	// 时间衰减
	ageDays := 365.0
	if v, ok := node.Metadata["last_accessed"].(float64); ok && v > 0 {
		lastAccess := time.Unix(int64(v), 0)
		ageHours := time.Since(lastAccess).Hours()
		ageDays = math.Max(ageHours/24, 1)
	}

	timeDecay := 1.0 / math.Log2(ageDays+1)

	// 综合评分
	if freq > 0 {
		return utility * timeDecay
	}
	return node.Score * timeDecay
}

// GCStats 垃圾回收统计
type GCStats struct {
	TotalNodes     int     `json:"total_nodes"`
	DeletedNodes   int     `json:"deleted_nodes"`
	ProtectedNodes int     `json:"protected_nodes"`
	AvgUtility     float64 `json:"avg_utility"`
	AvgFrequency   float64 `json:"avg_frequency"`
	Thresholds     struct {
		Utility float64 `json:"utility"`
		Freq    int     `json:"freq"`
	} `json:"thresholds"`
	LastRun time.Time `json:"last_run"`
}

// Stats 返回 GC 统计
func (c *MemoryCollector) Stats(ctx context.Context) (*GCStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Store == nil {
		return nil, nil
	}

	nodes, err := c.Store.Search(ctx, "", RetrieveOptions{
		TopK:     10000,
		MinScore: 0,
	})
	if err != nil {
		return nil, err
	}

	stats := &GCStats{
		TotalNodes: len(nodes),
		Thresholds: struct {
			Utility float64 `json:"utility"`
			Freq    int     `json:"freq"`
		}{
			Utility: c.UtilityThreshold,
			Freq:    c.FreqThreshold,
		},
	}

	var totalUtility, totalFreq float64
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if c.isProtected(n) {
			stats.ProtectedNodes++
		}
		totalUtility += floatVal(n.Metadata, "utility")
		totalFreq += float64(intVal(n.Metadata, "freq"))
	}

	if len(nodes) > 0 {
		stats.AvgUtility = totalUtility / float64(len(nodes))
		stats.AvgFrequency = totalFreq / float64(len(nodes))
	}

	return stats, nil
}

func intVal(metadata map[string]any, key string) int {
	if v, ok := metadata[key]; ok {
		switch vv := v.(type) {
		case float64:
			return int(vv)
		case int:
			return vv
		}
	}
	return 0
}

func floatVal(metadata map[string]any, key string) float64 {
	if v, ok := metadata[key]; ok {
		switch vv := v.(type) {
		case float64:
			return vv
		case int:
			return float64(vv)
		}
	}
	return 0
}

func int64Val(metadata map[string]any, key string) int64 {
	if v, ok := metadata[key]; ok {
		switch vv := v.(type) {
		case float64:
			return int64(vv)
		case int64:
			return vv
		case int:
			return int64(vv)
		}
	}
	return 0
}
