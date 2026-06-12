package memory

import (
	"fmt"
	"sync"
)

// Registry 泛型组件注册中心，用于注册和获取 Memory 相关组件（如 VectorStore、EmbeddingModel）。
// 轻量工厂模式，支持多后端（local/pgvector/qdrant 等）。
type Registry[T any] struct {
	mu      sync.RWMutex
	entries map[string]func() T
}

// NewRegistry 创建注册中心
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		entries: make(map[string]func() T),
	}
}

// Register 注册组件工厂函数
func (r *Registry[T]) Register(name string, factory func() T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = factory
}

// Get 按名称获取组件实例
func (r *Registry[T]) Get(name string) (T, error) {
	r.mu.RLock()
	factory, ok := r.entries[name]
	r.mu.RUnlock()
	if !ok {
		var zero T
		return zero, fmt.Errorf("registry: %q not found", name)
	}
	return factory(), nil
}

// Names 返回所有注册的名称
func (r *Registry[T]) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	return names
}

// Has 检查名称是否已注册
func (r *Registry[T]) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[name]
	return ok
}

// 全局注册中心实例
var (
	VectorStores    = NewRegistry[VectorStore]()
	EmbeddingModels = NewRegistry[EmbeddingModel]()
)
