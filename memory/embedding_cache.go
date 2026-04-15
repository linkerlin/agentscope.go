package memory

import (
	"context"
	"container/list"
	"crypto/sha256"
	"fmt"
	"sync"
)

// EmbeddingCache 带 LRU 缓存的 EmbeddingModel 包装器
type EmbeddingCache struct {
	embed  EmbeddingModel
	mu     sync.RWMutex
	cache  map[string]*list.Element
	lru    *list.List
	limit  int
	hits   uint64
	misses uint64
}

type cacheEntry struct {
	key   string
	value []float32
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
}

func (c *EmbeddingCache) hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum)
}

// Stats 返回缓存命中/未命中统计
func (c *EmbeddingCache) Stats() (hits, misses uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}

func dupVec(v []float32) []float32 {
	out := make([]float32, len(v))
	copy(out, v)
	return out
}

var _ EmbeddingModel = (*EmbeddingCache)(nil)
