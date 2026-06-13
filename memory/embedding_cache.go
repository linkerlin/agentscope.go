package memory

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EmbeddingCache 带 LRU 缓存的 EmbeddingModel 包装器，支持磁盘持久化
type EmbeddingCache struct {
	embed    EmbeddingModel
	mu       sync.RWMutex
	cache    map[string]*list.Element
	lru      *list.List
	limit    int
	hits     uint64
	misses   uint64
	diskPath string
	dirty    bool
}

type cacheEntry struct {
	key   string
	value []float32
}

type diskEntry struct {
	Key   string    `json:"key"`
	Value []float32 `json:"value"`
}

// NewEmbeddingCache 创建缓存包装器；limit<=0 时默认 1024
func NewEmbeddingCache(embed EmbeddingModel, limit int) *EmbeddingCache {
	if limit <= 0 {
		limit = 1024
	}
	return &EmbeddingCache{
		embed: embed,
		cache: make(map[string]*list.Element, limit),
		lru:   list.New(),
		limit: limit,
	}
}

// SetDiskPath 设置磁盘持久化路径，启用懒惰刷盘
func (c *EmbeddingCache) SetDiskPath(path string) {
	c.diskPath = path
}

// LoadFromDisk 从 JSONL 文件加载缓存
func (c *EmbeddingCache) LoadFromDisk() error {
	if c.diskPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.diskPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var entries []diskEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("embedding cache load: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range entries {
		if len(e.Value) == 0 {
			continue
		}
		if _, ok := c.cache[e.Key]; ok {
			continue
		}
		if c.lru.Len() >= c.limit {
			back := c.lru.Back()
			if back != nil {
				c.lru.Remove(back)
				delete(c.cache, back.Value.(*cacheEntry).key)
			}
		}
		elem := c.lru.PushFront(&cacheEntry{key: e.Key, value: dupVec(e.Value)})
		c.cache[e.Key] = elem
	}
	return nil
}

// SaveToDisk 将当前缓存写入 JSONL 文件
func (c *EmbeddingCache) SaveToDisk() error {
	if c.diskPath == "" {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	entries := make([]diskEntry, 0, len(c.cache))
	for key, elem := range c.cache {
		entries = append(entries, diskEntry{
			Key:   key,
			Value: elem.Value.(*cacheEntry).value,
		})
		_ = key
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.diskPath, data, 0o600) //nolint:gosec // G306: cache file, 0600 for safety
}

// Embed 单条嵌入（带缓存）
func (c *EmbeddingCache) Embed(ctx context.Context, text string) ([]float32, error) {
	key := c.hash(text)
	c.mu.RLock()
	if elem, ok := c.cache[key]; ok {
		c.lru.MoveToFront(elem)
		c.hits++
		c.mu.RUnlock()
		return dupVec(elem.Value.(*cacheEntry).value), nil
	}
	c.mu.RUnlock()

	v, err := c.embed.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	c.setLocked(key, v)
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
	return dupVec(v), nil
}

// EmbedBatch 批量嵌入（带缓存）
func (c *EmbeddingCache) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	var missIdx []int
	var missTexts []string

	c.mu.RLock()
	for i, text := range texts {
		key := c.hash(text)
		if elem, ok := c.cache[key]; ok {
			c.lru.MoveToFront(elem)
			out[i] = dupVec(elem.Value.(*cacheEntry).value)
			c.hits++
		} else {
			missIdx = append(missIdx, i)
			missTexts = append(missTexts, text)
		}
	}
	c.mu.RUnlock()

	if len(missTexts) > 0 {
		vecs, err := c.embed.EmbedBatch(ctx, missTexts)
		if err != nil {
			return nil, err
		}
		for i, idx := range missIdx {
			c.setLocked(c.hash(missTexts[i]), vecs[i])
			out[idx] = dupVec(vecs[i])
		}
		c.mu.Lock()
		c.misses += uint64(len(missTexts))
		c.mu.Unlock()
	}
	return out, nil
}

func (c *EmbeddingCache) setLocked(key string, value []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		c.lru.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = value
		return
	}
	if c.lru.Len() >= c.limit {
		back := c.lru.Back()
		if back != nil {
			c.lru.Remove(back)
			delete(c.cache, back.Value.(*cacheEntry).key)
		}
	}
	elem := c.lru.PushFront(&cacheEntry{key: key, value: value})
	c.cache[key] = elem
	c.dirty = true
}

func (c *EmbeddingCache) hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum)
}

// Stats 返回缓存命中/未命中统计 + 命中率
func (c *EmbeddingCache) Stats() (hits, misses uint64, hitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}
	return c.hits, c.misses, hitRate
}

// HitRate 返回缓存命中率
func (c *EmbeddingCache) HitRate() float64 {
	_, _, rate := c.Stats()
	return rate
}

// Size 返回当前缓存大小
func (c *EmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// Limit 返回缓存容量限制
func (c *EmbeddingCache) Limit() int {
	return c.limit
}

// Preload 预加载指定文本的嵌入（异步预热）
func (c *EmbeddingCache) Preload(ctx context.Context, texts []string) {
	go func() {
		_, _ = c.EmbedBatch(ctx, texts)
	}()
}

// Invalidate 使指定文本的缓存失效
func (c *EmbeddingCache) Invalidate(text string) {
	key := c.hash(text)
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		c.lru.Remove(elem)
		delete(c.cache, key)
		c.dirty = true
	}
}

// InvalidateAll 清空所有缓存
func (c *EmbeddingCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*list.Element, c.limit)
	c.lru = list.New()
	c.dirty = true
}

// Flush 强制刷盘
func (c *EmbeddingCache) Flush() error {
	return c.SaveToDisk()
}

// AutoSave 启动自动保存定时器
func (c *EmbeddingCache) AutoSave(interval int) {
	if c.diskPath == "" || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.mu.RLock()
			isDirty := c.dirty
			c.mu.RUnlock()
			if isDirty {
				_ = c.SaveToDisk()
				c.mu.Lock()
				c.dirty = false
				c.mu.Unlock()
			}
		}
	}()
}

// Report 生成缓存统计报告
func (c *EmbeddingCache) Report() string {
	hits, misses, rate := c.Stats()
	size := c.Size()
	return fmt.Sprintf(
		"EmbeddingCache Report:\n"+
		"  Size: %d/%d (%.1f%%)\n"+
		"  Hits: %d, Misses: %d\n"+
		"  Hit Rate: %.2f%%\n"+
		"  Disk Path: %s\n",
		size, c.limit, float64(size)/float64(c.limit)*100,
		hits, misses,
		rate*100,
		c.diskPath,
	)
}

func dupVec(v []float32) []float32 {
	out := make([]float32, len(v))
	copy(out, v)
	return out
}

var _ EmbeddingModel = (*EmbeddingCache)(nil)
