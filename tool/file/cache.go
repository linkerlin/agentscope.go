package file

import (
	"sync"
	"time"
)

const (
	defaultCacheMaxFiles = 100
	defaultCacheMaxKB    = 25
)

type cacheEntry struct {
	content []byte
	modTime time.Time
	size    int
}

// ReadCache is an LRU cache for file read results keyed by path.
type ReadCache struct {
	mu       sync.Mutex
	maxFiles int
	maxBytes int
	order    []string
	items    map[string]cacheEntry
	total    int
}

func NewReadCache(maxFiles, maxKB int) *ReadCache {
	if maxFiles <= 0 {
		maxFiles = defaultCacheMaxFiles
	}
	if maxKB <= 0 {
		maxKB = defaultCacheMaxKB
	}
	return &ReadCache{
		maxFiles: maxFiles,
		maxBytes: maxKB * 1024,
		items:    make(map[string]cacheEntry),
	}
}

func (c *ReadCache) Get(path string, modTime time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[path]
	if !ok || !e.modTime.Equal(modTime) {
		return nil, false
	}
	c.touch(path)
	return append([]byte(nil), e.content...), true
}

func (c *ReadCache) Put(path string, modTime time.Time, content []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.items[path]; ok {
		c.total -= old.size
	}
	size := len(content)
	c.items[path] = cacheEntry{content: append([]byte(nil), content...), modTime: modTime, size: size}
	c.total += size
	c.touch(path)
	c.evict()
}

func (c *ReadCache) touch(path string) {
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, path)
}

func (c *ReadCache) evict() {
	for (len(c.order) > c.maxFiles || c.total > c.maxBytes) && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		if e, ok := c.items[oldest]; ok {
			c.total -= e.size
			delete(c.items, oldest)
		}
	}
}

var defaultReadCache = NewReadCache(defaultCacheMaxFiles, defaultCacheMaxKB)
