package memory

import (
	"context"
	"math"
	"sync"
	"time"
)

// MemoryCollector 基于使用频率和效用的记忆自动清理器。
// 对标 ReMe Python delete_task_memory 流水线。
type MemoryCollector struct {
	Store             VectorStore
	FreqThreshold     int     // 检索频率达到此阈值时才考虑删除
	UtilityThreshold  float64 // utility/freq 低于此值则删除
	MaxAge            time.Duration // 最大保留时间，超时自动删除
	mu                sync.Mutex
}

// NewMemoryCollector 创建记忆清理器
func NewMemoryCollector(store VectorStore) *MemoryCollector {
	return &MemoryCollector{
		Store:            store,
		FreqThreshold:    3,
		UtilityThreshold: 0.2,
		MaxAge:           30 * 24 * time.Hour,
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

// Collect 扫描并删除低价值记忆
// 返回被删除的 memoryID 列表
func (c *MemoryCollector) Collect(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Store == nil {
		return nil, nil
	}

	// 调用 list 获取全部节点
	// 通过空查询 + 0 分获取全量
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

		freq := intVal(n.Metadata, "freq")
		utility := floatVal(n.Metadata, "utility")

		// 检查是否过期
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
