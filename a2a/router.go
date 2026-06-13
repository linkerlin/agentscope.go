package a2a

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ShardRouter routes requests to healthy A2A agents using consistent hashing.
// It supports virtual replicas per node for better distribution and failover.
type ShardRouter struct {
	registry *Registry
	replicas int

	mu      sync.RWMutex
	ring    []uint64
	nodes   map[uint64]string
	nodeSet map[string]struct{}
}

// NewShardRouter creates a router for the given registry.
// replicas controls how many virtual nodes each agent URL gets on the hash ring.
func NewShardRouter(registry *Registry, replicas int) *ShardRouter {
	if replicas <= 0 {
		replicas = 150
	}
	return &ShardRouter{
		registry: registry,
		replicas: replicas,
		nodes:    make(map[uint64]string),
		nodeSet:  make(map[string]struct{}),
	}
}

// Refresh rebuilds the hash ring from the current healthy entries in the registry.
func (r *ShardRouter) Refresh() error {
	entries := r.registry.List()
	ring := make([]uint64, 0, len(entries)*r.replicas)
	nodes := make(map[uint64]string)
	nodeSet := make(map[string]struct{})

	for _, e := range entries {
		if !e.Healthy {
			continue
		}
		url := e.Card.URL
		if url == "" {
			continue
		}
		nodeSet[url] = struct{}{}
		for i := 0; i < r.replicas; i++ {
			h := hashKey(fmt.Sprintf("%s:%d", url, i))
			ring = append(ring, h)
			nodes[h] = url
		}
	}

	sort.Slice(ring, func(i, j int) bool { return ring[i] < ring[j] })

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ring = ring
	r.nodes = nodes
	r.nodeSet = nodeSet
	return nil
}

// Route returns the agent URL responsible for the given key.
func (r *ShardRouter) Route(key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.ring) == 0 {
		return "", errors.New("a2a router: empty ring")
	}
	h := hashKey(key)
	idx := sort.Search(len(r.ring), func(i int) bool { return r.ring[i] >= h })
	if idx == len(r.ring) {
		idx = 0
	}
	node, ok := r.nodes[r.ring[idx]]
	if !ok {
		return "", errors.New("a2a router: node not found")
	}
	return node, nil
}

// HasNode reports whether url is currently on the ring.
func (r *ShardRouter) HasNode(url string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.nodeSet[url]
	return ok
}

// AutoRefresh starts a goroutine that refreshes the hash ring whenever the
// set of healthy agents changes. It reacts immediately to local Watch events
// and also polls at the given interval to catch external store changes (e.g.
// another process updating Redis). Stop the goroutine by cancelling ctx.
func (r *ShardRouter) AutoRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		previous := r.healthySet()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		watchCh := r.registry.Watch(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-watchCh:
				// Local change: refresh unconditionally and update baseline.
				_ = r.Refresh()
				previous = r.healthySet()
			case <-ticker.C:
				current := r.healthySetFromRegistry()
				if !setsEqual(previous, current) {
					_ = r.Refresh()
					previous = current
				}
			}
		}
	}()
}

// healthySet returns the currently cached healthy node set.
func (r *ShardRouter) healthySet() map[string]struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]struct{}, len(r.nodeSet))
	for url := range r.nodeSet {
		out[url] = struct{}{}
	}
	return out
}

// healthySetFromRegistry computes the healthy node set directly from registry.
func (r *ShardRouter) healthySetFromRegistry() map[string]struct{} {
	entries := r.registry.List()
	out := make(map[string]struct{})
	for _, e := range entries {
		if e.Healthy && e.Card.URL != "" {
			out[e.Card.URL] = struct{}{}
		}
	}
	return out
}

func setsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// hashKey returns a 64-bit hash for consistent hashing.
func hashKey(key string) uint64 {
	sum := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(sum[:8])
}
